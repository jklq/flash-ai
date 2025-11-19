import type { UserCard, Sprint, TopicName } from "../types";

const SPRINT_SIZE = 10;
const MIN_NEW_CARDS = 3;
const TOPIC_ORDER: TopicName[] = [
  "Processes",
  "Memory",
  "Concurrency",
  "Storage",
];

export const getNextTopic = (currentTopic: TopicName | null): TopicName => {
  if (!currentTopic) return TOPIC_ORDER[0];
  const currentIndex = TOPIC_ORDER.indexOf(currentTopic);
  const nextIndex = (currentIndex + 1) % TOPIC_ORDER.length;
  return TOPIC_ORDER[nextIndex];
};

export const createSprint = (
  topic: TopicName,
  allCards: UserCard[]
): Sprint => {
  const topicCards = allCards.filter((c) => c.topic === topic);
  const now = new Date();

  // 1. Due cards (scheduled for today or earlier)
  const dueCards = topicCards
    .filter(
      (c) => c.fsrsCard.due.getTime() <= now.getTime() && c.fsrsCard.reps > 0
    )
    .sort((a, b) => a.fsrsCard.due.getTime() - b.fsrsCard.due.getTime());

  // 2. New cards (never studied)
  const newCards = topicCards.filter((c) => c.fsrsCard.reps === 0);

  // 3. Fill sprint
  let sprintCards: UserCard[] = [];

  // Determine how many due cards to take
  // We want to leave room for MIN_NEW_CARDS if we have new cards available
  const maxDueCards =
    newCards.length > 0
      ? Math.max(0, SPRINT_SIZE - Math.min(newCards.length, MIN_NEW_CARDS))
      : SPRINT_SIZE;

  // Prioritize due cards, but respect the limit to allow new cards
  sprintCards = dueCards.slice(0, maxDueCards);

  // Fill remaining slots with new cards
  if (sprintCards.length < SPRINT_SIZE) {
    const needed = SPRINT_SIZE - sprintCards.length;
    sprintCards = [...sprintCards, ...newCards.slice(0, needed)];
  }

  // If still not full, fill with review ahead (future cards)
  if (sprintCards.length < SPRINT_SIZE) {
    const needed = SPRINT_SIZE - sprintCards.length;
    // Future cards: reps > 0 and due > now
    const futureCards = topicCards
      .filter(
        (c) => c.fsrsCard.reps > 0 && c.fsrsCard.due.getTime() > now.getTime()
      )
      .sort((a, b) => a.fsrsCard.due.getTime() - b.fsrsCard.due.getTime());

    sprintCards = [...sprintCards, ...futureCards.slice(0, needed)];
  }

  // If we still don't have 10, maybe add some review ahead? Or just smaller sprint.
  // For now, just limit to SPRINT_SIZE if we have too many due cards
  if (sprintCards.length > SPRINT_SIZE) {
    sprintCards = sprintCards.slice(0, SPRINT_SIZE);
  }

  return {
    topic,
    cards: sprintCards,
    currentIndex: 0,
    completed: false,
  };
};
