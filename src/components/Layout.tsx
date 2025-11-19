import React from 'react';
import { LayoutGrid, BookOpen } from 'lucide-react';
import clsx from 'clsx';

interface LayoutProps {
    children: React.ReactNode;
    currentView: 'review' | 'overview';
    onViewChange: (view: 'review' | 'overview') => void;
}

export const Layout: React.FC<LayoutProps> = ({ children, currentView, onViewChange }) => {
    return (
        <div className="min-h-screen bg-slate-50 flex flex-col">
            <header className="bg-white border-b border-slate-200 sticky top-0 z-10">
                <div className="max-w-5xl mx-auto px-4 h-16 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <div className="w-8 h-8 bg-brand-600 rounded-lg flex items-center justify-center text-white font-bold">
                            F
                        </div>
                        <span className="font-bold text-xl tracking-tight text-slate-900">FlashAI</span>
                    </div>

                    <nav className="flex gap-1 bg-slate-100 p-1 rounded-lg">
                        <button
                            onClick={() => onViewChange('review')}
                            className={clsx(
                                "px-4 py-1.5 rounded-md text-sm font-medium transition-all flex items-center gap-2",
                                currentView === 'review'
                                    ? "bg-white text-brand-700 shadow-sm"
                                    : "text-slate-600 hover:text-slate-900 hover:bg-slate-200/50"
                            )}
                        >
                            <BookOpen size={16} />
                            Review
                        </button>
                        <button
                            onClick={() => onViewChange('overview')}
                            className={clsx(
                                "px-4 py-1.5 rounded-md text-sm font-medium transition-all flex items-center gap-2",
                                currentView === 'overview'
                                    ? "bg-white text-brand-700 shadow-sm"
                                    : "text-slate-600 hover:text-slate-900 hover:bg-slate-200/50"
                            )}
                        >
                            <LayoutGrid size={16} />
                            Overview
                        </button>
                    </nav>
                </div>
            </header>

            <main className="flex-1 max-w-5xl mx-auto w-full p-4 md:p-8">
                {children}
            </main>
        </div>
    );
};
