import React, { useState } from 'react';
import { motion } from 'framer-motion';

import type { UserCard } from '../types';
import { Rating } from '../services/fsrs';

interface FlashcardProps {
    card: UserCard;
    onRate: (rating: Rating) => void;
    onFlipChange?: (isFlipped: boolean) => void;
    isChatExpanded?: boolean;
}

export const Flashcard: React.FC<FlashcardProps> = ({ card, onRate, onFlipChange, isChatExpanded }) => {
    const [isFlipped, setIsFlipped] = useState(false);

    // Reset flip state when card changes
    React.useEffect(() => {
        setIsFlipped(false);
        onFlipChange?.(false);
    }, [card.id]);

    const handleFlip = () => {
        if (!isFlipped) {
            setIsFlipped(true);
            onFlipChange?.(true);
        }
    };

    React.useEffect(() => {
        const handleKeyDown = (event: KeyboardEvent) => {
            // Ignore if typing in an input or textarea
            const target = event.target as HTMLElement;
            if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
                return;
            }

            if (event.code === 'Space') {
                event.preventDefault();
                handleFlip();
            }

            if (isFlipped && !isChatExpanded) {
                switch (event.key) {
                    case '1':
                        onRate(Rating.Again);
                        break;
                    case '2':
                        onRate(Rating.Hard);
                        break;
                    case '3':
                        onRate(Rating.Good);
                        break;
                    case '4':
                        onRate(Rating.Easy);
                        break;
                }
            }
        };

        window.addEventListener('keydown', handleKeyDown);
        return () => window.removeEventListener('keydown', handleKeyDown);
    }, [isFlipped, onRate, onFlipChange, isChatExpanded]);

    return (
        <div className="w-full max-w-2xl mx-auto perspective-1000 h-[400px]">
            <motion.div
                className="relative w-full h-full transform-style-3d cursor-pointer"
                initial={false}
                animate={{ rotateY: isFlipped ? 180 : 0 }}
                transition={{ duration: 0.6, type: "spring", stiffness: 260, damping: 20 }}
                onClick={handleFlip}
                style={{ transformStyle: 'preserve-3d', WebkitTransformStyle: 'preserve-3d' }}
            >
                {/* Front */}
                <div
                    className="absolute w-full h-full backface-hidden bg-white rounded-2xl shadow-xl border border-slate-100 p-8 flex flex-col items-center justify-center text-center"
                    style={{ backfaceVisibility: 'hidden', WebkitBackfaceVisibility: 'hidden' }}
                >
                    <span className="absolute top-6 left-6 text-xs font-semibold uppercase tracking-wider text-slate-400">
                        {card.topic}
                    </span>
                    <h3 className="text-2xl md:text-3xl font-medium text-slate-800 leading-relaxed">
                        {card.question}
                    </h3>
                    <p className="absolute bottom-6 text-sm text-slate-400 font-medium animate-pulse">
                        Click to reveal answer
                    </p>
                </div>

                {/* Back */}
                <div
                    className="absolute w-full h-full backface-hidden rotate-y-180 bg-white rounded-2xl shadow-xl border border-slate-100 p-8 flex flex-col items-center justify-center text-center"
                    style={{ backfaceVisibility: 'hidden', WebkitBackfaceVisibility: 'hidden' }}
                >
                    <div className="flex-1 flex flex-col items-center justify-center gap-6">
                        <div className="w-full border-b border-slate-100 pb-4 mb-2">
                            <p className="text-sm font-medium text-slate-400 uppercase tracking-wider mb-2">Question</p>
                            <p className="text-lg text-slate-600 font-medium">
                                {card.question}
                            </p>
                        </div>

                        <div className="flex-1 flex items-center justify-center">
                            <p className="text-xl md:text-2xl text-slate-800 leading-relaxed font-semibold">
                                {card.answer}
                            </p>
                        </div>
                    </div>

                    <div className="w-full grid grid-cols-4 gap-3 mt-8" onClick={(e) => e.stopPropagation()}>
                        <button
                            onClick={() => onRate(Rating.Again)}
                            className="py-3 px-2 rounded-xl bg-red-50 text-red-600 hover:bg-red-500 hover:text-white font-medium transition-colors text-sm md:text-base border border-red-100 hover:border-red-500"
                        >
                            Again
                        </button>
                        <button
                            onClick={() => onRate(Rating.Hard)}
                            className="py-3 px-2 rounded-xl bg-orange-50 text-orange-600 hover:bg-orange-500 hover:text-white font-medium transition-colors text-sm md:text-base border border-orange-100 hover:border-orange-500"
                        >
                            Hard
                        </button>
                        <button
                            onClick={() => onRate(Rating.Good)}
                            className="py-3 px-2 rounded-xl bg-blue-50 text-blue-600 hover:bg-blue-500 hover:text-white font-medium transition-colors text-sm md:text-base border border-blue-100 hover:border-blue-500"
                        >
                            Good
                        </button>
                        <button
                            onClick={() => onRate(Rating.Easy)}
                            className="py-3 px-2 rounded-xl bg-emerald-50 text-emerald-600 hover:bg-emerald-500 hover:text-white font-medium transition-colors text-sm md:text-base border border-emerald-100 hover:border-emerald-500"
                        >
                            Easy
                        </button>
                    </div>
                </div>
            </motion.div>
        </div>
    );
};
