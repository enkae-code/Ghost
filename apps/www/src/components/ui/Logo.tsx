import { motion } from 'framer-motion';

/**
 * Logo (Hollow Prism)
 * 
 * A geometric, rotating prism that symbolizes the "Ghost in the Machine".
 * Uses Framer Motion for SVG path drawing and rotation.
 */
export const Logo = ({ className = "w-10 h-10" }: { className?: string }) => {
  return (
    <motion.svg
      viewBox="0 0 100 100"
      className={className}
      initial={{ rotate: 0 }}
      animate={{ rotate: 360 }}
      transition={{ duration: 20, repeat: Infinity, ease: "linear" }}
    >
      {/* Outer Prism (Diamond) */}
      <motion.path
        d="M50 5 L95 50 L50 95 L5 50 Z"
        fill="transparent"
        stroke="currentColor"
        strokeWidth="2"
        initial={{ pathLength: 0, opacity: 0 }}
        animate={{ pathLength: 1, opacity: 1 }}
        transition={{ duration: 2, ease: "easeInOut" }}
      />
      
      {/* Inner Core (Triangle) */}
      <motion.path
        d="M50 25 L75 75 H25 Z"
        fill="transparent"
        stroke="currentColor"
        strokeWidth="1"
        initial={{ pathLength: 0, opacity: 0 }}
        animate={{ pathLength: 1, opacity: 0.8 }}
        transition={{ duration: 2.5, delay: 0.5, ease: "easeInOut" }}
      />
      
      {/* Ghost Eye (Circle) */}
      <motion.circle
        cx="50"
        cy="45"
        r="4"
        fill="currentColor"
        initial={{ scale: 0 }}
        animate={{ scale: [0, 1.2, 1] }}
        transition={{ duration: 0.5, delay: 2.5 }}
      />
    </motion.svg>
  );
};
