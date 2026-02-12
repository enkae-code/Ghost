import { useEffect, useState, useRef } from 'react';

/**
 * DecryptedText
 * 
 * A text animation that reveals characters by decoding random glyphs.
 * Based on the "Hollow Prism" design direction.
 */

interface DecryptedTextProps {
  text: string;
  speed?: number;
  maxIterations?: number;
  className?: string;
  characters?: string;
}

const DEFAULT_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890!@#$%^&*()_+-=[]{}|;:,.<>?';

export const DecryptedText = ({
  text,
  speed = 50,
  maxIterations = 10,
  className = '',
  characters = DEFAULT_CHARS,
}: DecryptedTextProps) => {
  const [displayText, setDisplayText] = useState(text);
  const [isHovering, setIsHovering] = useState(false);
  const iterationRef = useRef(0);
  const intervalRef = useRef<any>(null);

  const handleMouseEnter = () => setIsHovering(true);
  const handleMouseLeave = () => {
    setIsHovering(false);
    setDisplayText(text);
    if (intervalRef.current) clearInterval(intervalRef.current);
  };

  useEffect(() => {
    if (!isHovering) return;

    iterationRef.current = 0;

    intervalRef.current = setInterval(() => {
      setDisplayText(() => {
        const currentIter = iterationRef.current;
        
        if (currentIter >= text.length) {
          if (intervalRef.current) clearInterval(intervalRef.current);
          return text;
        }

        const newText = text
          .split('')
          .map((char, index) => {
            if (index < currentIter) {
              return text[index];
            }
            return characters[Math.floor(Math.random() * characters.length)];
          })
          .join('');
        
        iterationRef.current += 1 / maxIterations;
        return newText;
      });
    }, speed);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [isHovering, text, speed, maxIterations, characters]);

  return (
    <span
      className={className}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      {displayText}
    </span>
  );
};
