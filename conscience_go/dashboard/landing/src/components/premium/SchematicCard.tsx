import { motion, useMotionTemplate, useMotionValue } from "framer-motion";
import type { MouseEvent, ReactNode } from "react";
import { cn } from "../../lib/utils";

interface SchematicCardProps {
  children: ReactNode;
  className?: string;
  title?: string;
  label?: string;
}

export const SchematicCard = ({ children, className, title, label }: SchematicCardProps) => {
  const mouseX = useMotionValue(0);
  const mouseY = useMotionValue(0);

  function onMouseMove({ currentTarget, clientX, clientY }: MouseEvent<HTMLDivElement>) {
    const { left, top } = currentTarget.getBoundingClientRect();
    mouseX.set(clientX - left);
    mouseY.set(clientY - top);
  }

  return (
    <div
      className={cn(
        "group relative border border-white/10 bg-plate overflow-hidden",
        "hover:border-white/20 transition-colors duration-500",
        className
      )}
      onMouseMove={onMouseMove}
    >
      {/* Grid Pattern Overlay */}
      <div className="absolute inset-0 bg-grid opacity-20 bg-size-[14px_24px] mask-[radial-gradient(ellipse_60%_50%_at_50%_0%,#000_70%,transparent_100%)] pointer-events-none" />

      {/* Spotlight Effect */}
      <motion.div
        className="pointer-events-none absolute -inset-px rounded-xl opacity-0 transition duration-300 group-hover:opacity-100"
        style={{
          background: useMotionTemplate`
            radial-gradient(
              650px circle at ${mouseX}px ${mouseY}px,
              rgba(0, 240, 255, 0.1),
              transparent 80%
            )
          `,
        }}
      />

      {/* Crosshair Markers (Technical feel) */}
      <div className="absolute top-0 left-0 w-2 h-2 border-t border-l border-cyan/50" />
      <div className="absolute top-0 right-0 w-2 h-2 border-t border-r border-cyan/50" />
      <div className="absolute bottom-0 left-0 w-2 h-2 border-b border-l border-cyan/50" />
      <div className="absolute bottom-0 right-0 w-2 h-2 border-b border-r border-cyan/50" />

      {/* Content */}
      <div className="relative min-h-[inherit] flex flex-col grow">
        {title && (
            <div className="flex justify-between items-start mb-4 border-b border-white/5 pb-2">
                <h3 className="text-lg font-medium text-ghost font-sans tracking-tight">{title}</h3>
                {label && <span className="text-label text-cyan/50">{label}</span>}
            </div>
        )}
        <div className="grow text-ghost/70 font-light leading-relaxed">
            {children}
        </div>
      </div>
    </div>
  );
};
