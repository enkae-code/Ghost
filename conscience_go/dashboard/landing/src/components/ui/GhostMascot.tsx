import { useEffect, useRef, useState } from 'react';
import { motion } from 'framer-motion';

export const GhostMascot = () => {
    const ghostRef = useRef<HTMLDivElement>(null);
    const [mousePosition, setMousePosition] = useState({ x: 0, y: 0 });

    useEffect(() => {
        const handleMouseMove = (event: MouseEvent) => {
            if (!ghostRef.current) return;
            const rect = ghostRef.current.getBoundingClientRect();
            const centerX = rect.left + rect.width / 2;
            const centerY = rect.top + rect.height / 2;
            
            // Calculate normalized position (-1 to 1)
            const x = (event.clientX - centerX) / (window.innerWidth / 2);
            const y = (event.clientY - centerY) / (window.innerHeight / 2);
            
            setMousePosition({ x, y });
        };

        window.addEventListener('mousemove', handleMouseMove);
        return () => window.removeEventListener('mousemove', handleMouseMove);
    }, []);

    return (
        <div className="relative w-64 h-64 flex items-center justify-center" ref={ghostRef}>
            {/* Glow Effect (Subtle) */}
            <div className="absolute inset-0 bg-cyan/10 blur-[60px] rounded-full scale-150 animate-pulse" />

            {/* Ghost Entity (No Container) */}
            <motion.div 
                className="relative flex flex-col items-center justify-center p-8"
                animate={{
                    y: [0, -10, 0],
                }}
                transition={{
                    duration: 4,
                    repeat: Infinity,
                    ease: "easeInOut"
                }}
            >
                {/* Eyes Container */}
                <div className="flex gap-8 relative z-10">
                    {/* Left Eye */}
                    <div className="w-12 h-16 bg-void rounded-full relative overflow-hidden border border-cyan/20 shadow-[0_0_15px_rgba(0,229,255,0.1)]">
                         <motion.div 
                            className="absolute w-4 h-4 bg-cyan rounded-full top-3 left-3 shadow-[0_0_10px_rgba(0,229,255,0.8)]"
                            animate={{
                                x: mousePosition.x * 12,
                                y: mousePosition.y * 12
                            }}
                            transition={{ type: "spring", stiffness: 150, damping: 15 }}
                         />
                    </div>
                    {/* Right Eye */}
                    <div className="w-12 h-16 bg-void rounded-full relative overflow-hidden border border-cyan/20 shadow-[0_0_15px_rgba(0,229,255,0.1)]">
                        <motion.div 
                            className="absolute w-4 h-4 bg-cyan rounded-full top-3 left-3 shadow-[0_0_10px_rgba(0,229,255,0.8)]"
                            animate={{
                                x: mousePosition.x * 12,
                                y: mousePosition.y * 12
                            }}
                            transition={{ type: "spring", stiffness: 150, damping: 15 }}
                         />
                    </div>
                </div>


            </motion.div>
        </div>
    );
};
