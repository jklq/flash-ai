import React, { useState, useRef, useEffect } from 'react';
import { motion } from 'framer-motion';
import { Send, RotateCcw, Sparkles, ChevronUp, ChevronDown, Settings, Maximize2, Minimize2, Zap } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage, UserCard } from '../types';
import { sendChatMessage } from '../services/openrouter';

interface ChatWindowProps {
    isVisible?: boolean;
    onReset?: () => void;
    currentCard?: UserCard;
    triggerExpand?: boolean;
    expanded?: boolean;
    onExpandChange?: (expanded: boolean) => void;
}

const MarkdownRenderer = React.memo(({ content, isUser }: { content: string, isUser: boolean }) => (
    <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
            p: ({ children }) => <p className="mb-2 last:mb-0 leading-relaxed">{children}</p>,
            ul: ({ children }) => <ul className="list-disc list-outside ml-4 mb-2 space-y-1">{children}</ul>,
            ol: ({ children }) => <ol className="list-decimal list-outside ml-4 mb-2 space-y-1">{children}</ol>,
            li: ({ children }) => <li className="pl-1">{children}</li>,
            h1: ({ children }) => <h1 className="text-lg font-bold mb-2 mt-4">{children}</h1>,
            h2: ({ children }) => <h2 className="text-base font-bold mb-2 mt-3">{children}</h2>,
            h3: ({ children }) => <h3 className="text-sm font-bold mb-1 mt-2">{children}</h3>,
            blockquote: ({ children }) => (
                <blockquote className={`border-l-4 pl-3 italic my-2 ${isUser ? 'border-white/30' : 'border-slate-300'}`}>
                    {children}
                </blockquote>
            ),
            code: ({ node, className, children, ...props }: any) => {
                return (
                    <code 
                        className={`${className || ''} font-mono text-sm ${
                             isUser 
                                ? '[&:not(pre_&)]:bg-white/20 [&:not(pre_&)]:text-white [&:not(pre_&)]:px-1.5 [&:not(pre_&)]:py-0.5 [&:not(pre_&)]:rounded' 
                                : '[&:not(pre_&)]:bg-slate-200 [&:not(pre_&)]:text-slate-800 [&:not(pre_&)]:px-1.5 [&:not(pre_&)]:py-0.5 [&:not(pre_&)]:rounded'
                        }`} 
                        {...props}
                    >
                        {children}
                    </code>
                );
            },
            pre: ({ children }) => (
                <div className="rounded-lg overflow-hidden my-2">
                    <pre className={`p-3 overflow-x-auto text-sm font-mono ${isUser ? 'bg-black/30 text-white' : 'bg-slate-900 text-slate-50'}`}>
                        {children}
                    </pre>
                </div>
            ),
            a: ({ children, href }) => (
                <a 
                    href={href} 
                    target="_blank" 
                    rel="noopener noreferrer" 
                    className={`underline underline-offset-2 ${isUser ? 'text-white hover:text-white/80' : 'text-brand-600 hover:text-brand-700'}`}
                >
                    {children}
                </a>
            ),
            table: ({ children }) => (
                <div className="overflow-x-auto my-2 rounded-lg border border-slate-200/50">
                    <table className="min-w-full divide-y divide-slate-200/50">
                        {children}
                    </table>
                </div>
            ),
            thead: ({ children }) => <thead className={isUser ? 'bg-white/10' : 'bg-slate-50'}>{children}</thead>,
            th: ({ children }) => <th className="px-3 py-2 text-left text-xs font-medium uppercase tracking-wider opacity-70">{children}</th>,
            td: ({ children }) => <td className="px-3 py-2 text-sm border-t border-slate-200/20">{children}</td>,
        }}
    >
        {content}
    </ReactMarkdown>
));

export const ChatWindow: React.FC<ChatWindowProps> = ({ 
    isVisible = true, 
    onReset, 
    currentCard, 
    triggerExpand,
    expanded: controlledExpanded,
    onExpandChange
}) => {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [isLoading, setIsLoading] = useState(false);
    const [streamingMessage, setStreamingMessage] = useState('');
    const [internalExpanded, setInternalExpanded] = useState(false);
    
    const isExpanded = controlledExpanded ?? internalExpanded;
    const setIsExpanded = (value: boolean) => {
        if (onExpandChange) {
            onExpandChange(value);
        } else {
            setInternalExpanded(value);
        }
    };

    const [autoExpand, setAutoExpand] = useState(() => {
        const saved = localStorage.getItem("chat_auto_expand");
        return saved ? JSON.parse(saved) : false;
    });
    const [apiKey, setApiKey] = useState(localStorage.getItem("openrouter_api_key") || '');
    const [showSettings, setShowSettings] = useState(false);
    const [isFullscreen, setIsFullscreen] = useState(false);
    
    const streamingContentRef = useRef('');
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    useEffect(() => {
        scrollToBottom();
    }, [messages, streamingMessage, isExpanded, isFullscreen]);

    // Focus input when chat becomes expanded
    useEffect(() => {
        if ((isExpanded || isFullscreen) && inputRef.current) {
            inputRef.current.focus();
        }
    }, [isExpanded, isFullscreen]);

    // Handle auto-expand
    useEffect(() => {
        if (triggerExpand && autoExpand) {
            setIsExpanded(true);
        }
    }, [triggerExpand, autoExpand]);

    // Save auto-expand preference
    useEffect(() => {
        localStorage.setItem("chat_auto_expand", JSON.stringify(autoExpand));
    }, [autoExpand]);

    // Reset chat when card changes
    useEffect(() => {
        if (currentCard?.id) {
            setMessages([]);
            setStreamingMessage('');
            streamingContentRef.current = '';
            setInput('');
            onReset?.();
            if (autoExpand) {
                setIsExpanded(false);
            }
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [currentCard?.id]);

    const handleReset = (e?: React.MouseEvent) => {
        e?.stopPropagation();
        setMessages([]);
        setStreamingMessage('');
        streamingContentRef.current = '';
        setInput('');
        onReset?.();
    };

    const handleSaveKey = (e: React.FormEvent) => {
        e.preventDefault();
        if (input.trim()) {
            localStorage.setItem("openrouter_api_key", input.trim());
            setApiKey(input.trim());
            setInput('');
            setShowSettings(false);
        }
    };

    const handleClearKey = () => {
        localStorage.removeItem("openrouter_api_key");
        setApiKey('');
        setMessages([]);
        setShowSettings(false);
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!input.trim() || isLoading) return;

        const userMessage: ChatMessage = {
            role: 'user',
            content: input.trim()
        };

        // Optimistically add user message
        setMessages(prev => [...prev, userMessage]);
        setInput('');
        setIsLoading(true);
        setStreamingMessage('');
        streamingContentRef.current = '';

        const allMessages = [...messages, userMessage];
        
        let apiMessages = allMessages;
        if (currentCard) {
            const systemMessage: ChatMessage = {
                role: 'system',
                content: `You are a helpful study assistant. The user is studying a flashcard.
Question: ${currentCard.question}
Answer: ${currentCard.answer}
Topic: ${currentCard.topic}
Please help the user understand this concept. Be concise and encouraging.`
            };
            apiMessages = [systemMessage, ...allMessages];
        }

        try {
            await sendChatMessage(apiMessages, {
                onToken: (token) => {
                    streamingContentRef.current += token;
                    setStreamingMessage(streamingContentRef.current);
                },
                onComplete: () => {
                    const content = streamingContentRef.current;
                    setMessages(prev => [...prev, {
                        role: 'assistant',
                        content
                    }]);
                    setStreamingMessage('');
                    streamingContentRef.current = '';
                    setIsLoading(false);
                },
                onError: (error) => {
                    console.error('Chat error:', error);
                    setMessages(prev => [...prev, {
                        role: 'assistant',
                        content: `Error: ${error.message}`
                    }]);
                    setStreamingMessage('');
                    streamingContentRef.current = '';
                    setIsLoading(false);
                }
            });
        } catch (error) {
            console.error('Chat submission error:', error);
            setIsLoading(false);
        }
    };

    if (!isVisible) return null;

    return (
        <div className={`fixed z-50 flex flex-col pointer-events-none transition-all duration-300 ${
            isFullscreen 
                ? 'inset-0 items-center justify-center bg-black/20 backdrop-blur-sm p-4' 
                : 'right-6 bottom-6 items-end'
        }`}>
            <motion.div
                initial={false}
                animate={{ 
                    height: isFullscreen ? '100%' : (isExpanded ? 500 : 'auto'),
                    width: isFullscreen ? '100%' : '24rem',
                }}
                transition={{ type: 'spring', damping: 40, stiffness: 300 }}
                className={`bg-white/95 backdrop-blur-xl shadow-2xl border border-slate-200 flex flex-col overflow-hidden pointer-events-auto ${
                    isFullscreen ? 'rounded-2xl w-full h-full max-w-5xl max-h-[90vh]' : 'rounded-2xl'
                }`}
            >
                {/* Header */}
                <div 
                    className="px-6 py-4 border-b border-slate-200 bg-linear-to-r from-brand-50 to-purple-50 cursor-pointer hover:bg-slate-50 transition-colors"
                    onClick={() => {
                        if (isFullscreen) {
                            setIsFullscreen(false);
                        } else {
                            setIsExpanded(!isExpanded);
                        }
                    }}
                >
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <div className="w-8 h-8 rounded-lg bg-linear-to-br from-brand-500 to-purple-500 flex items-center justify-center">
                                <Sparkles size={18} className="text-white" />
                            </div>
                            <h3 className="font-semibold text-slate-800">AI</h3>
                        </div>
                        <div className="flex items-center gap-3">
                            {!isFullscreen && (
                                <motion.button
                                    whileHover={{ scale: 1.1 }}
                                    whileTap={{ scale: 0.9 }}
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        setAutoExpand(!autoExpand);
                                    }}
                                    className={`p-1.5 rounded-lg transition-colors duration-200 ${
                                        autoExpand 
                                            ? 'bg-amber-100 text-amber-600' 
                                            : 'text-slate-400 hover:bg-white/50 hover:text-slate-600'
                                    }`}
                                    title="Auto Toggle"
                                >
                                    <motion.div
                                        animate={autoExpand ? { 
                                            scale: [1, 1.2, 1],
                                            rotate: [0, 15, -15, 0]
                                        } : {}}
                                        transition={{ duration: 0.4 }}
                                    >
                                        <Zap 
                                            size={16} 
                                            className={autoExpand ? 'fill-current' : ''} 
                                        />
                                    </motion.div>
                                </motion.button>
                            )}
                            <button
                                onClick={(e) => {
                                    e.stopPropagation();
                                    if (!isFullscreen) {
                                        setIsExpanded(true);
                                    }
                                    setIsFullscreen(!isFullscreen);
                                }}
                                className="p-1.5 hover:bg-white/50 rounded-lg transition-colors text-slate-600 hover:text-brand-600"
                                title={isFullscreen ? "Exit fullscreen" : "Fullscreen"}
                            >
                                {isFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
                            </button>
                            <button
                                onClick={(e) => {
                                    e.stopPropagation();
                                    setInput('');
                                    setShowSettings(!showSettings);
                                }}
                                className={`p-1.5 hover:bg-white/50 rounded-lg transition-colors ${showSettings ? 'text-brand-600 bg-white/50' : 'text-slate-600 hover:text-brand-600'}`}
                                title="Settings"
                            >
                                <Settings size={16} />
                            </button>
                            <button
                                onClick={handleReset}
                                className="p-1.5 hover:bg-white/50 rounded-lg transition-colors text-slate-600 hover:text-brand-600"
                                title="Reset conversation"
                            >
                                <RotateCcw size={16} />
                            </button>
                            <div className="text-slate-400">
                                {(isExpanded || isFullscreen) ? <ChevronDown size={18} /> : <ChevronUp size={18} />}
                            </div>
                        </div>
                    </div>
                </div>

                {/* Content */}
                {(isExpanded || isFullscreen) && (
                    <motion.div 
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ duration: 0.2, delay: 0.1 }}
                        className="flex-1 flex flex-col min-h-0"
                    >
                        {(!apiKey || showSettings) ? (
                            <div className="flex-1 p-6 flex flex-col justify-center items-center text-center space-y-4">
                                <div className="w-12 h-12 rounded-xl bg-brand-100 text-brand-600 flex items-center justify-center mb-2">
                                    <Settings size={24} />
                                </div>
                                <h3 className="font-semibold text-slate-800">OpenRouter API Key</h3>
                                <p className="text-sm text-slate-600">
                                    To use the study assistant, you need to provide your own OpenRouter API key.
                                    It will be stored locally in your browser.
                                </p>
                                <form onSubmit={handleSaveKey} className="w-full space-y-3">
                                    <input
                                        type="password"
                                        value={input}
                                        onChange={(e) => setInput(e.target.value)}
                                        placeholder="sk-or-..."
                                        className="w-full px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent text-sm"
                                    />
                                    <button
                                        type="submit"
                                        disabled={!input.trim()}
                                        className="w-full px-4 py-2.5 bg-brand-600 text-white rounded-xl hover:bg-brand-700 transition-colors font-medium disabled:opacity-50 disabled:cursor-not-allowed"
                                    >
                                        Save API Key
                                    </button>
                                </form>
                                {apiKey && (
                                    <button
                                        onClick={handleClearKey}
                                        className="text-xs text-red-500 hover:text-red-600 underline"
                                    >
                                        Remove saved key
                                    </button>
                                )}
                                <div className="text-xs text-slate-400">
                                    Get a key at <a href="https://openrouter.ai/keys" target="_blank" rel="noopener noreferrer" className="text-brand-600 hover:underline">openrouter.ai</a>
                                </div>
                            </div>
                        ) : (
                            <>
                                {/* Messages */}
                                <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
                                    {messages.length === 0 && !streamingMessage && (
                                        <div className="text-center text-slate-400 mt-8">
                                            <p className="text-sm">Ask me anything about this flashcard!</p>
                                        </div>
                                    )}

                                    {messages.map((msg, idx) => (
                                        <div
                                            key={idx}
                                            className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
                                        >
                                            <div
                                                className={`max-w-[80%] rounded-2xl px-4 py-2.5 ${msg.role === 'user'
                                                        ? 'bg-linear-to-br from-brand-500 to-brand-600 text-white'
                                                        : 'bg-slate-100 text-slate-800'
                                                    }`}
                                            >
                                                <MarkdownRenderer content={msg.content} isUser={msg.role === 'user'} />
                                            </div>
                                        </div>
                                    ))}

                                    {streamingMessage && (
                                        <motion.div
                                            initial={{ opacity: 0, y: 10 }}
                                            animate={{ opacity: 1, y: 0 }}
                                            className="flex justify-start"
                                        >
                                            <div className="max-w-[80%] rounded-2xl px-4 py-2.5 bg-slate-100 text-slate-800">
                                                <MarkdownRenderer content={streamingMessage} isUser={false} />
                                                <span className="inline-block w-1 h-4 bg-slate-400 ml-1 animate-pulse align-middle" />
                                            </div>
                                        </motion.div>
                                    )}

                                    <div ref={messagesEndRef} />
                                </div>

                                {/* Input */}
                                <div className="p-4 border-t border-slate-200 bg-white">
                                    <form onSubmit={handleSubmit} className="flex gap-2">
                                        <input
                                            ref={inputRef}
                                            type="text"
                                            value={input}
                                            onChange={(e) => setInput(e.target.value)}
                                            placeholder="Ask a question..."
                                            disabled={isLoading}
                                            className="flex-1 px-4 py-2.5 rounded-xl border border-slate-200 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent disabled:bg-slate-50 disabled:text-slate-400 text-sm"
                                        />
                                        <button
                                            type="submit"
                                            disabled={!input.trim() || isLoading}
                                            className="px-4 py-2.5 bg-linear-to-r from-brand-500 to-brand-600 text-white rounded-xl hover:from-brand-600 hover:to-brand-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg shadow-brand-200 disabled:shadow-none"
                                        >
                                            <Send size={18} />
                                        </button>
                                    </form>
                                </div>
                            </>
                        )}
                    </motion.div>
                )}
            </motion.div>
        </div>
    );
};
