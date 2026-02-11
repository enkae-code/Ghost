import { motion } from 'framer-motion';
import { cn } from '../../lib/utils';
import type { ReactNode } from 'react';

interface BentoCardProps {
    children: ReactNode;
    className?: string;
    colSpan?: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12;
    title?: string;
    subtitle?: string;
}

export const BentoCard = ({ children, className, colSpan = 3, title, subtitle }: BentoCardProps) => {
    // Map colSpan to Tailwind grid classes
    const colSpanClass = {
        1: "col-span-1",
        2: "col-span-1 md:col-span-2",
        3: "col-span-1 md:col-span-3",
        4: "col-span-1 md:col-span-2 lg:col-span-4",
        5: "col-span-1 md:col-span-5",
        6: "col-span-1 md:col-span-3 lg:col-span-6",
        7: "col-span-7",
        8: "col-span-8",
        9: "col-span-9",
        10: "col-span-10",
        11: "col-span-11",
        12: "col-span-12",
    }[colSpan] || "col-span-3";

    return (
        <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: "-100px" }}
            transition={{ duration: 0.5, ease: "easeOut" }}
            className={cn(
                "group relative overflow-hidden rounded-2xl bg-surface border border-white/5 p-6 backdrop-blur-sm",
                "hover:border-white/10 transition-colors duration-300",
                colSpanClass,
                className
            )}
        >
             {/* Gradient Shine on Hover */}
             <div className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-700 pointer-events-none">
                 <div className="absolute inset-0 bg-gradient-to-tr from-transparent via-cyan/5 to-transparent skew-x-12 translate-x-[-100%] group-hover:translate-x-[100%] transition-transform duration-1000" />
             </div>

            <div className="relative z-10 flex flex-col h-full">
                {title && (
                    <div className="mb-4">
                        <h3 className="text-xl font-medium text-ghost font-sans tracking-tight">{title}</h3>
                        {subtitle && <p className="text-sm text-ghost/60 font-mono mt-1">{subtitle}</p>}
                    </div>
                )}
                <div className="flex-grow">
                    {children}
                </div>
            </div>
        </motion.div>
    );
};
