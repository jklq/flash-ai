import Papa from 'papaparse';
import type { FlashcardData, TopicName } from '../types';

const TOPICS: TopicName[] = ['Processes', 'Memory', 'Concurrency', 'Storage'];

export const loadAllCards = async (): Promise<FlashcardData[]> => {
    const allCards: FlashcardData[] = [];

    for (const topic of TOPICS) {
        try {
            const response = await fetch(`/data/${topic.toLowerCase()}.csv`);
            const csvText = await response.text();

            const { data } = Papa.parse(csvText, {
                header: true,
                skipEmptyLines: true,
            });

            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            const topicCards = data.map((row: any, index: number) => ({
                id: `${topic}-${index}`,
                question: row.Question,
                answer: row.Answer,
                topic: topic,
            }));

            // Shuffle cards within the topic
            for (let i = topicCards.length - 1; i > 0; i--) {
                const j = Math.floor(Math.random() * (i + 1));
                [topicCards[i], topicCards[j]] = [topicCards[j], topicCards[i]];
            }

            allCards.push(...topicCards);
        } catch (error) {
            console.error(`Failed to load cards for topic ${topic}:`, error);
        }
    }

    return allCards;
};
