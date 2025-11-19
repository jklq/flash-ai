import type { Card as FSRSCard, ReviewLog as FSRSReviewLog } from "ts-fsrs";

export type TopicName = "Processes" | "Memory" | "Concurrency" | "Storage";

export interface FlashcardData {
  id: string;
  question: string;
  answer: string;
  topic: TopicName;
}

export interface UserCard extends FlashcardData {
  fsrsCard: FSRSCard;
  logs: FSRSReviewLog[];
}

export interface Sprint {
  topic: TopicName;
  cards: UserCard[];
  currentIndex: number;
  completed: boolean;
}

export interface TopicStats {
  topic: TopicName;
  averageStability: number;
  retentionRate: number;
  totalReviews: number;
  lapsedCount: number; // How many times cards in this topic were forgotten
}

export interface ChatMessage {
  role: "user" | "assistant" | "system";
  content: string;
}
