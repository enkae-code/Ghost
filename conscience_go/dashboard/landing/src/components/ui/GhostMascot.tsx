import React, { useEffect, useState } from 'react';
import { motion, useAnimation } from 'framer-motion';

/**
 * THE RESONANT GHOST
 * ------------------
 * A programmatic representation of the Ghost entity.
 * Not a mascot. A signal.
 * * DESIGN SPECS:
 * - Body: Geometric Capsule (Squircle), Matte Ghost White (#F8F8FF).
 * - Eyes: "Baymax" Trust Standard. No pupils. Tracking physics.
 * - Tail: Procedural Sine Wave (The Voice).
 */

export const GhostMascot = () => {
  const [mousePosition, setMousePosition] = useState({ x: 0, y: 0 });
  const controls = useAnimation();

  // Handle Eye Tracking (The Watchful Guardian)
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      // Calculate normalized position (-1 to 1) for eye movement
      const x = (e.clientX / window.innerWidth) * 2 - 1;
      const y = (e.clientY / window.innerHeight) * 2 - 1;
      setMousePosition({ x, y });
    };

    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, []);

  // Eye movement physics constraints (max movement in pixels)
  const eyeX = mousePosition.x * 6;
  const eyeY = mousePosition.y * 6;

  return (
    <div className="relative w-64 h-64 flex items-center justify-center">
      {/* Container: Floating Animation */}
      <motion.div
        animate={{
          y: [0, -8, 0],
        }}
        transition={{
          duration: 4,
          repeat: Infinity,
          ease: "easeInOut"
        }}
        className="relative z-10"
      >
        <svg
          width="160"
          height="180"
          viewBox="0 0 160 180"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
          className="drop-shadow-[0_0_15px_rgba(100,255,218,0.1)]" // Subtle Cyan Glow
        >
          {/* THE BODY: Geometric Capsule */}
          <path
            d="M 20 80 
               C 20 30, 140 30, 140 80 
               V 130 
               C 140 130, 140 130, 140 130
               L 140 140
               C 140 140, 110 130, 80 130 
               C 50 130, 20 140, 20 140 
               Z"
            fill="#F8F8FF" // Ghost White
            stroke="#0A192F" // Void Blue border for high contrast
            strokeWidth="0" 
          />

          {/* THE TAIL: The Signal (Waveform Overlay) 
              This replaces the "ragged sheet" with a data stream.
          */}
          <motion.path
            d="M 20 140 
               C 40 140, 50 155, 80 145 
               C 110 135, 120 150, 140 140 
               L 140 130 
               C 110 120, 50 120, 20 130 
               Z"
            fill="#64FFDA" // Cyan Signal
            initial={{ opacity: 0.8 }}
            animate={{
              d: [
                // Wave State 1
                "M 20 140 C 40 140, 50 155, 80 145 C 110 135, 120 150, 140 140 L 140 130 C 110 120, 50 120, 20 130 Z",
                // Wave State 2 (Inverted)
                "M 20 140 C 40 150, 50 135, 80 145 C 110 155, 120 130, 140 140 L 140 130 C 110 120, 50 120, 20 130 Z",
                // Wave State 1
                "M 20 140 C 40 140, 50 155, 80 145 C 110 135, 120 150, 140 140 L 140 130 C 110 120, 50 120, 20 130 Z"
              ]
            }}
            transition={{
              duration: 2,
              repeat: Infinity,
              ease: "easeInOut"
            }}
          />

          {/* THE EYES: Geometric Void */}
          <g transform="translate(0, 0)">
            {/* Left Eye */}
            <motion.ellipse 
              cx="55" 
              cy="75" 
              rx="8" 
              ry="12" 
              fill="#0A192F"
              animate={{ x: eyeX, y: eyeY }}
              transition={{ type: "spring", stiffness: 150, damping: 15 }}
            />
            {/* Right Eye */}
            <motion.ellipse 
              cx="105" 
              cy="75" 
              rx="8" 
              ry="12" 
              fill="#0A192F"
              animate={{ x: eyeX, y: eyeY }}
              transition={{ type: "spring", stiffness: 150, damping: 15 }}
            />
          </g>
        </svg>
      </motion.div>

      {/* Grounding Shadow (Optional: Adds depth) */}
      <motion.div
        className="absolute bottom-10 w-20 h-2 bg-black/20 rounded-full blur-sm"
        animate={{
          scale: [1, 0.8, 1],
          opacity: [0.2, 0.1, 0.2]
        }}
        transition={{
          duration: 4,
          repeat: Infinity,
          ease: "easeInOut"
        }}
      />
    </div>
  );
};
