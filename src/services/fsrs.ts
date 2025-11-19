import { FSRS, Rating, generatorParameters, createEmptyCard } from 'ts-fsrs';
import type { Card } from 'ts-fsrs';
import type { UserCard } from '../types';

const params = generatorParameters({ enable_fuzz: true });
const fsrs = new FSRS(params);

export const createNewUserCard = (cardData: any): UserCard => {
    return {
        ...cardData,
        fsrsCard: createEmptyCard(),
        logs: [],
    };
};

export const scheduleCard = (
    userCard: UserCard,
    rating: Rating
): { card: Card; log: any } => {
    const schedulingCards = fsrs.repeat(userCard.fsrsCard, new Date());
    const { card, log } = (schedulingCards as any)[rating];
    return { card, log };
};

export { Rating };
