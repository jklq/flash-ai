import { useState, useEffect, useMemo } from 'react';
import { Layout } from './components/Layout';
import { SprintView } from './components/SprintView';
import { Overview } from './components/Overview';
import { loadAllCards } from './services/data';
import { createNewUserCard, scheduleCard, Rating } from './services/fsrs';
import { createSprint, getNextTopic } from './services/sprint';
import type { UserCard, Sprint, TopicName, TopicStats } from './types';

function App() {
  const [view, setView] = useState<'review' | 'overview'>('review');
  const [cards, setCards] = useState<UserCard[]>([]);
  const [currentSprint, setCurrentSprint] = useState<Sprint | null>(null);
  const [loading, setLoading] = useState(true);

  // Load initial data
  useEffect(() => {
    const init = async () => {
      const savedData = localStorage.getItem('flash-ai-cards');
      const savedSprint = localStorage.getItem('flash-ai-sprint');
      
      let finalCards: UserCard[] = [];

      if (savedData) {
        try {
          const parsedCards = JSON.parse(savedData, (key, value) => {
            // Revive date strings to Date objects
            if (key === 'due' || key === 'last_review' || key === 'review') {
              return new Date(value);
            }
            return value;
          });
          finalCards = parsedCards;
        } catch (e) {
          console.error("Failed to parse saved data, falling back to fresh load", e);
        }
      }

      if (finalCards.length === 0) {
        // Fallback to fresh load
        const rawCards = await loadAllCards();
        finalCards = rawCards.map(createNewUserCard);
      }

      setCards(finalCards);

      if (savedSprint) {
        try {
          const parsedSprint = JSON.parse(savedSprint, (key, value) => {
            // Revive date strings to Date objects
            if (key === 'due' || key === 'last_review' || key === 'review') {
              return new Date(value);
            }
            return value;
          });

          // Sync sprint cards with global cards to ensure they are up to date
          parsedSprint.cards = parsedSprint.cards.map((sprintCard: UserCard) => {
            const globalCard = finalCards.find(c => c.id === sprintCard.id);
            return globalCard || sprintCard;
          });

          setCurrentSprint(parsedSprint);
        } catch (e) {
          console.error("Failed to parse saved sprint", e);
        }
      }

      setLoading(false);
    };
    init();
  }, []);

  // Save to localStorage whenever cards change
  useEffect(() => {
    if (!loading && cards.length > 0) {
      localStorage.setItem('flash-ai-cards', JSON.stringify(cards));
    }
  }, [cards, loading]);

  // Save sprint to localStorage whenever it changes
  useEffect(() => {
    if (!loading) {
      if (currentSprint) {
        localStorage.setItem('flash-ai-sprint', JSON.stringify(currentSprint));
      } else {
        localStorage.removeItem('flash-ai-sprint');
      }
    }
  }, [currentSprint, loading]);

  // Start first sprint when cards are loaded
  useEffect(() => {
    if (!loading && cards.length > 0 && !currentSprint) {
      startNewSprint(null);
    }
  }, [loading, cards]);

  const startNewSprint = (lastTopic: TopicName | null) => {
    const nextTopic = getNextTopic(lastTopic);
    const sprint = createSprint(nextTopic, cards);
    setCurrentSprint(sprint);
  };

  const handleRateCard = (card: UserCard, rating: Rating) => {
    const { card: newFsrsCard, log } = scheduleCard(card, rating);

    // Update card in global state
    const updatedCard = {
      ...card,
      fsrsCard: newFsrsCard,
      logs: [...card.logs, log]
    };

    setCards(prev => prev.map(c => c.id === card.id ? updatedCard : c));

    // Update sprint state
    if (currentSprint) {
      if (rating === Rating.Again) {
        const newCards = [...currentSprint.cards];
        newCards.splice(currentSprint.currentIndex, 1);
        newCards.push(updatedCard);

        setCurrentSprint({
          ...currentSprint,
          cards: newCards,
        });
      } else {
        const nextIndex = currentSprint.currentIndex + 1;
        const isComplete = nextIndex >= currentSprint.cards.length;

        // Update the card in the sprint array to reflect the new state
        const newCards = [...currentSprint.cards];
        newCards[currentSprint.currentIndex] = updatedCard;

        setCurrentSprint({
          ...currentSprint,
          cards: newCards,
          currentIndex: nextIndex,
          completed: isComplete
        });
      }
    }
  };

  const handleSprintComplete = () => {
    if (currentSprint) {
      startNewSprint(currentSprint.topic);
    }
  };

  // Calculate stats for Overview
  const stats: TopicStats[] = useMemo(() => {
    const topics: TopicName[] = ['Processes', 'Memory', 'Concurrency', 'Storage'];
    return topics.map(topic => {
      const topicCards = cards.filter(c => c.topic === topic);
      const reviewedCards = topicCards.filter(c => c.fsrsCard.reps > 0);

      const totalReviews = reviewedCards.reduce((acc, c) => acc + c.fsrsCard.reps, 0);
      const lapsedCount = reviewedCards.reduce((acc, c) => acc + c.fsrsCard.lapses, 0);

      // Simple retention rate approximation: successful reviews / total reviews
      // Or just use stability as a proxy for "how well known"
      // Let's use average stability for now, normalized roughly
      const avgStability = reviewedCards.length > 0
        ? reviewedCards.reduce((acc, c) => acc + c.fsrsCard.stability, 0) / reviewedCards.length
        : 0;

      // Retention rate based on FSRS formula: R = 0.9 ^ (elapsed / stability)
      // But we want a general "mastery" score. Let's use a simple metric:
      // % of cards with stability > 3 days (arbitrary threshold for "learned")
      const learnedCards = reviewedCards.filter(c => c.fsrsCard.stability > 3).length;
      const retentionRate = topicCards.length > 0 ? learnedCards / topicCards.length : 0;

      return {
        topic,
        averageStability: avgStability,
        retentionRate,
        totalReviews,
        lapsedCount
      };
    });
  }, [cards]);

  const difficultCards = useMemo(() => {
    // Define difficult as: lapses > 0 OR stability < 1 (after at least 1 review)
    return cards
      .filter(c => (c.fsrsCard.lapses > 0 || (c.fsrsCard.reps > 0 && c.fsrsCard.stability < 1)))
      .sort((a, b) => a.fsrsCard.stability - b.fsrsCard.stability)
      .slice(0, 10); // Top 10 most difficult
  }, [cards]);

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-slate-50 text-slate-400">
        Loading...
      </div>
    );
  }

  return (
    <Layout currentView={view} onViewChange={setView}>
      {view === 'review' && currentSprint && (
        <SprintView
          sprint={currentSprint}
          onRateCard={handleRateCard}
          onComplete={handleSprintComplete}
        />
      )}
      {view === 'overview' && (
        <Overview stats={stats} difficultCards={difficultCards} />
      )}
    </Layout>
  );
}

export default App;
