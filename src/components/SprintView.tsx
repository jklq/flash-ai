import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ArrowRight, Trophy, CheckCircle } from 'lucide-react';
import type { Sprint, UserCard } from '../types';
import { Flashcard } from './Flashcard';
import { Rating } from '../services/fsrs';
import { ChatWindow } from './ChatWindow';


import confetti from 'canvas-confetti';

interface SprintViewProps {
    sprint: Sprint;
    onRateCard: (card: UserCard, rating: Rating) => void;
    onComplete: () => void;
}

export const SprintView: React.FC<SprintViewProps> = ({ sprint, onRateCard, onComplete }) => {
    const [isCardFlipped, setIsCardFlipped] = useState(false);
    const [isChatExpanded, setIsChatExpanded] = useState(false);
    const [lastRating, setLastRating] = useState<Rating | null>(null);
    const currentCard = sprint.cards[sprint.currentIndex];
    const progress = ((sprint.currentIndex) / sprint.cards.length) * 100;

    const handleRate = (rating: Rating) => {
        setLastRating(rating);
        onRateCard(currentCard, rating);
    };

    React.useEffect(() => {
        if (sprint.completed) {
            const duration = 3 * 1000;
            const animationEnd = Date.now() + duration;
            const defaults = { startVelocity: 45, spread: 360, ticks: 100, zIndex: 0 };

            const randomInRange = (min: number, max: number) => {
                return Math.random() * (max - min) + min;
            }

            const interval: any = setInterval(function () {
                const timeLeft = animationEnd - Date.now();

                if (timeLeft <= 0) {
                    return clearInterval(interval);
                }

                const particleCount = 100 * (timeLeft / duration);

                // since particles fall down, start a bit higher than random
                confetti({ ...defaults, particleCount, origin: { x: randomInRange(0.1, 0.3), y: Math.random() - 0.2 } });
                confetti({ ...defaults, particleCount, origin: { x: randomInRange(0.7, 0.9), y: Math.random() - 0.2 } });
            }, 250);

            return () => clearInterval(interval);
        }
    }, [sprint.completed]);

    if (sprint.completed) {
        return (
            <motion.div
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ opacity: 1, scale: 1 }}
                className="flex flex-col items-center justify-center min-h-[60vh] text-center"
            >
                <div className="w-24 h-24 bg-brand-100 rounded-full flex items-center justify-center mb-6 text-brand-600">
                    <Trophy size={48} />
                </div>
                <h2 className="text-3xl font-bold text-slate-900 mb-2">Sprint Complete!</h2>
                <p className="text-slate-500 mb-8 max-w-md">
                    Great job! You've completed your {sprint.topic} sprint. Ready for the next challenge?
                </p>
                <button
                    onClick={onComplete}
                    className="px-8 py-3 bg-brand-600 text-white rounded-xl font-semibold hover:bg-brand-700 transition-colors shadow-lg shadow-brand-200 flex items-center gap-2"
                >
                    Start Next Sprint <ArrowRight size={20} />
                </button>
            </motion.div>
        );
    }

    if (!currentCard) return null;

    const cardContentVariants = {
        center: { opacity: 1 },
        exit: (rating: Rating | null) => ({
            opacity: rating !== null && rating >= Rating.Hard ? 0 : 1,
            transition: { duration: 0.2 }
        })
    };

    const checkMarkVariants = {
        initial: { opacity: 0, scale: 0.5 },
        exit: (rating: Rating | null) => ({
            opacity: rating !== null && rating >= Rating.Hard ? 1 : 0,
            scale: rating !== null && rating >= Rating.Hard ? 1 : 0.5,
            transition: { duration: 0.2 }
        })
    };

    return (
        <>
            <div className="max-w-3xl mx-auto">
                <div className="mb-8">
                    <div className="flex justify-between text-sm font-medium text-slate-500 mb-2">
                        <span>{sprint.topic} Sprint</span>
                        <span>{sprint.currentIndex + 1} / {sprint.cards.length}</span>
                    </div>
                    <div className="h-2 bg-slate-100 rounded-full overflow-hidden">
                        <motion.div
                            className="h-full bg-brand-500"
                            initial={{ width: 0 }}
                            animate={{ width: `${progress}%` }}
                            transition={{ duration: 0.3 }}
                        />
                    </div>
                </div>

                <AnimatePresence mode='wait' custom={lastRating}>
                    <motion.div
                        key={`${currentCard.id}-${currentCard.logs.length}`}
                        custom={lastRating}
                        initial={{ x: 50, opacity: 0 }}
                        animate={{ x: 0, opacity: 1 }}
                        exit={{ x: -50, opacity: 0, transition: { duration: 0.2 } }}
                        transition={{ duration: 0.2 }}
                        className="relative"
                    >
                        <motion.div
                            variants={cardContentVariants}
                            initial="center"
                            animate="center"
                            exit="exit"
                            custom={lastRating}
                        >
                            <Flashcard
                                card={currentCard}
                                onRate={handleRate}
                                onFlipChange={setIsCardFlipped}
                                isChatExpanded={isChatExpanded}
                            />
                        </motion.div>

                        <motion.div
                            variants={checkMarkVariants}
                            initial="initial"
                            exit="exit"
                            custom={lastRating}
                            className="absolute inset-0 flex items-center justify-center pointer-events-none z-10"
                        >
                            <div className="bg-white rounded-full p-4 shadow-lg">
                                <CheckCircle className="text-green-500 w-24 h-24" />
                            </div>
                        </motion.div>
                    </motion.div>
                </AnimatePresence>
            </div>

            <ChatWindow
                isVisible={true}
                triggerExpand={isCardFlipped}
                currentCard={currentCard}
                expanded={isChatExpanded}
                onExpandChange={setIsChatExpanded}
            />
        </>
    );
};
