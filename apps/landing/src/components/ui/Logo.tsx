/**
 * Nav logo: same character as the hero GhostMascot (white body, dark eyes, wavy cyan base)
 * at smaller size. Exact same paths and viewBox as GhostMascot — no motion on paths
 * so Framer doesn't add pathLength/rotation that changes how it looks.
 */
export const Logo = ({ className = "w-10 h-10" }: { className?: string }) => {
  return (
    <div className={`relative ${className} flex items-center justify-center shrink-0 pointer-events-none md:pointer-events-auto`}>
      <svg
        viewBox="0 0 160 180"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        className="w-full h-full drop-shadow-[0_0_8px_rgba(100,255,218,0.2)]"
      >
        {/* Body: exact same path as GhostMascot */}
        <path
          d="M 20 80 
             C 20 30, 140 30, 140 80 
             V 130 
             C 140 130, 140 130, 140 130
             L 140 140
             C 140 140, 110 130, 80 130 
             C 50 130, 20 140, 20 140 
             Z"
          fill="#F8F8FF"
          strokeWidth="0"
        />

        {/* Tail: exact same path as GhostMascot (first frame of wave) */}
        <path
          d="M 20 140 
             C 40 140, 50 155, 80 145 
             C 110 135, 120 150, 140 140 
             L 140 130 
             C 110 120, 50 120, 20 130 
             Z"
          fill="#64FFDA"
        />

        {/* Eyes: same positions and size as GhostMascot */}
        <ellipse cx="55" cy="75" rx="8" ry="12" fill="#0A192F" />
        <ellipse cx="105" cy="75" rx="8" ry="12" fill="#0A192F" />
      </svg>
    </div>
  );
};
