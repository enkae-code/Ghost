import { motion } from 'framer-motion';
import { GhostMascot } from '../components/ui/GhostMascot';

export const Hero = () => {
    return (
        <section className="min-h-screen flex flex-col items-center justify-center relative overflow-hidden px-4 py-20">
            {/* Background elements (grid is in body, maybe add spotlilght here) */}
            
            <div className="z-10 flex flex-col items-center text-center max-w-5xl mx-auto space-y-12">
                
                {/* The Mascot */}
                <motion.div 
                    initial={{ opacity: 0, scale: 0.8 }}
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{ duration: 1.5, ease: [0.16, 1, 0.3, 1] }}
                >
                    <GhostMascot />
                </motion.div>

                {/* Main Headline */}
                <div className="space-y-4">
                    <motion.h1 
                        className="text-5xl md:text-7xl font-bold tracking-tighter text-ghost"
                        initial={{ opacity: 0, y: 20 }}
                        animate={{ opacity: 1, y: 0 }}
                        transition={{ delay: 0.5, duration: 1, ease: [0.16, 1, 0.3, 1] }}
                    >
                        The Digital <span className="text-transparent bg-clip-text bg-gradient-to-r from-ghost to-ghost/50">Prosthetic</span>
                    </motion.h1>

                    <motion.p 
                        className="text-xl md:text-2xl text-ghost/60 font-mono"
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        transition={{ delay: 0.8, duration: 1 }}
                    >
                        Sovereign Infrastructure for the Tri-Brain Architecture.
                    </motion.p>
                </div>

                {/* Status Indicator */}
                <motion.div
                    className="flex items-center gap-2 px-4 py-2 rounded-full bg-surface border border-white/10"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 1.2, duration: 1 }}
                >
                    <div className="w-2 h-2 rounded-full bg-cyan animate-pulse shadow-[0_0_10px_#00e5ff]" />
                    <span className="text-xs font-mono text-cyan tracking-widest uppercase">System Online</span>
                </motion.div>

            </div>
        </section>
    );
};
