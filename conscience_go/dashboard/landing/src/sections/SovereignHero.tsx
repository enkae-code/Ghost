import { motion } from 'framer-motion';
import { AbstractEntity } from '../components/ui/AbstractEntity';
import { MagneticButton } from '../components/premium/MagneticButton';

export const SovereignHero = () => {
    return (
        <section className="relative min-h-screen flex flex-col items-center justify-center overflow-hidden pt-20">
            
            {/* Background Ambience */}
            <div className="absolute inset-0 bg-void pointer-events-none" />
            
            {/* Abstract Entity (Center Stage) */}
            <motion.div 
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 1.5, ease: "easeOut" }}
                className="relative z-10 mb-12"
            >
                <AbstractEntity />
            </motion.div>

            {/* Typography */}
            <motion.div 
                className="relative z-20 text-center"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.8, delay: 0.5 }}
            >
                <h1 className="text-9xl md:text-[12rem] font-bold tracking-tighter text-transparent bg-clip-text bg-linear-to-b from-white to-white/10 leading-none select-none">
                    GHOST
                </h1>
                <p className="mt-6 text-sm md:text-base text-ghost/50 font-mono uppercase tracking-[0.2em]">
                    Sovereign Intelligence Layer v6.0
                </p>
            </motion.div>

            {/* Minimal CTAs */}
            <motion.div 
                className="mt-16 flex flex-col items-center gap-6 relative z-20"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ duration: 1, delay: 1 }}
            >
                <MagneticButton className="px-12 py-4 bg-white text-black hover:bg-cyan hover:text-black transition-colors" onClick={() => console.log('Download')}>
                    DOWNLOAD INSTALLER
                </MagneticButton>
                
                <div className="flex gap-8 text-xs font-mono text-ghost/30 uppercase tracking-widest">
                    <span className="hover:text-cyan cursor-pointer transition-colors">Documentation</span>
                    <span className="hover:text-cyan cursor-pointer transition-colors">Source Code</span>
                    <span className="hover:text-cyan cursor-pointer transition-colors">Manifesto</span>
                </div>
            </motion.div>

        </section>
    );
};
