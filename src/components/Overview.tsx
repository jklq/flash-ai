import React, { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronDown, ChevronUp, AlertCircle, BarChart3 } from 'lucide-react';
import type { TopicStats, UserCard } from '../types';
import clsx from 'clsx';

interface OverviewProps {
    stats: TopicStats[];
    difficultCards: UserCard[];
}

export const Overview: React.FC<OverviewProps> = ({ stats, difficultCards }) => {
    const [expandedCardId, setExpandedCardId] = useState<string | null>(null);

    // Sort stats by retention rate (lowest first to highlight areas for improvement)
    const sortedStats = [...stats].sort((a, b) => a.retentionRate - b.retentionRate);

    return (
        <div className="space-y-12">
            <section>
                <h2 className="text-2xl font-bold text-slate-900 mb-6 flex items-center gap-2">
                    <BarChart3 className="text-brand-600" />
                    Topic Performance
                </h2>
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                    {sortedStats.map((stat, index) => (
                        <motion.div
                            key={stat.topic}
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ delay: index * 0.1 }}
                            className="bg-white p-6 rounded-2xl shadow-sm border border-slate-100"
                        >
                            <h3 className="font-semibold text-slate-700 mb-2">{stat.topic}</h3>
                            <div className="flex items-end gap-2 mb-1">
                                <span className="text-3xl font-bold text-slate-900">
                                    {Math.round(stat.retentionRate * 100)}%
                                </span>
                                <span className="text-sm text-slate-500 mb-1">retention</span>
                            </div>
                            <div className="w-full bg-slate-100 h-1.5 rounded-full overflow-hidden">
                                <div
                                    className={clsx("h-full rounded-full",
                                        stat.retentionRate > 0.8 ? "bg-emerald-500" :
                                            stat.retentionRate > 0.5 ? "bg-yellow-500" : "bg-red-500"
                                    )}
                                    style={{ width: `${stat.retentionRate * 100}%` }}
                                />
                            </div>
                            <div className="mt-4 text-xs text-slate-400 flex justify-between">
                                <span>{stat.totalReviews} reviews</span>
                                <span>{stat.lapsedCount} lapses</span>
                            </div>
                        </motion.div>
                    ))}
                </div>
            </section>

            <section>
                <h2 className="text-2xl font-bold text-slate-900 mb-6 flex items-center gap-2">
                    <AlertCircle className="text-red-500" />
                    Difficult Questions
                </h2>

                {difficultCards.length === 0 ? (
                    <div className="text-center py-12 bg-white rounded-2xl border border-slate-100 text-slate-500">
                        No difficult questions identified yet. Keep studying!
                    </div>
                ) : (
                    <div className="space-y-3">
                        {difficultCards.map((card) => (
                            <div
                                key={card.id}
                                className="bg-white rounded-xl border border-slate-100 overflow-hidden transition-shadow hover:shadow-md"
                            >
                                <button
                                    onClick={() => setExpandedCardId(expandedCardId === card.id ? null : card.id)}
                                    className="w-full px-6 py-4 flex items-center justify-between text-left"
                                >
                                    <div className="flex items-center gap-4">
                                        <span className="px-2 py-1 bg-slate-100 text-slate-600 text-xs font-medium rounded-md uppercase tracking-wide">
                                            {card.topic}
                                        </span>
                                        <span className="font-medium text-slate-800 line-clamp-1">
                                            {card.question}
                                        </span>
                                    </div>
                                    {expandedCardId === card.id ? (
                                        <ChevronUp className="text-slate-400" size={20} />
                                    ) : (
                                        <ChevronDown className="text-slate-400" size={20} />
                                    )}
                                </button>

                                <AnimatePresence>
                                    {expandedCardId === card.id && (
                                        <motion.div
                                            initial={{ height: 0, opacity: 0 }}
                                            animate={{ height: "auto", opacity: 1 }}
                                            exit={{ height: 0, opacity: 0 }}
                                            className="border-t border-slate-50 bg-slate-50/50"
                                        >
                                            <div className="px-6 py-4 text-slate-600">
                                                <p className="mb-2 font-medium text-slate-900">Answer:</p>
                                                <p>{card.answer}</p>
                                                <div className="mt-4 flex gap-4 text-xs text-slate-400">
                                                    <span>Stability: {card.fsrsCard.stability.toFixed(2)}</span>
                                                    <span>Difficulty: {card.fsrsCard.difficulty.toFixed(2)}</span>
                                                    <span>Lapses: {card.fsrsCard.lapses}</span>
                                                </div>
                                            </div>
                                        </motion.div>
                                    )}
                                </AnimatePresence>
                            </div>
                        ))}
                    </div>
                )}
            </section>
        </div>
    );
};
