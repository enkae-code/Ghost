import { useRef, useState } from "react";
import type { MouseEvent, ReactNode } from "react";
import { motion } from "framer-motion";
import { cn } from "../../lib/utils";

interface MagneticButtonProps {
  children: ReactNode;
  className?: string;
  onClick?: () => void;
}

export const MagneticButton = ({ children, className, onClick }: MagneticButtonProps) => {
  const ref = useRef<HTMLButtonElement>(null);
  const [position, setPosition] = useState({ x: 0, y: 0 });

  const handleMouse = (e: MouseEvent<HTMLButtonElement>) => {
    const { clientX, clientY } = e;
    const rect = ref.current?.getBoundingClientRect();
    
    if (rect) {
      const { height, width, left, top } = rect;
      const middleX = clientX - (left + width / 2);
      const middleY = clientY - (top + height / 2);
      setPosition({ x: middleX * 0.1, y: middleY * 0.1 });
    }
  };

  const reset = () => {
    setPosition({ x: 0, y: 0 });
  };

  const { x, y } = position;

  return (
    <motion.button
      ref={ref}
      className={cn(
        "relative px-6 py-3 text-sm font-medium uppercase tracking-widest text-cyan border border-cyan/30 bg-cyan/5 overflow-hidden group",
        "hover:bg-cyan/10 hover:border-cyan/50 transition-colors duration-300",
        className
      )}
      animate={{ x, y }}
      transition={{ type: "spring", stiffness: 150, damping: 15, mass: 0.1 }}
      onMouseMove={handleMouse}
      onMouseLeave={reset}
      onClick={onClick}
    >
      <span className="relative z-10 flex items-center gap-2">
        {children}
      </span>
      
      {/* Hover Fill Effect */}
      <span className="absolute inset-0 bg-cyan/10 translate-y-full group-hover:translate-y-0 transition-transform duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]" />
    </motion.button>
  );
};
