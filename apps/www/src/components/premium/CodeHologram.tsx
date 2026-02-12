import { motion, useMotionValue, useSpring, useTransform } from "framer-motion";
import { useEffect } from "react";

export const CodeHologram = () => {
  const x = useMotionValue(0);
  const y = useMotionValue(0);

  const rotateX = useSpring(useTransform(y, [-300, 300], [10, -10]));
  const rotateY = useSpring(useTransform(x, [-300, 300], [-10, 10]));

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      const { innerWidth, innerHeight } = window;
      x.set(e.clientX - innerWidth / 2);
      y.set(e.clientY - innerHeight / 2);
    };

    window.addEventListener("mousemove", handleMouseMove);
    return () => window.removeEventListener("mousemove", handleMouseMove);
  }, [x, y]);

  return (
    <div className="perspective-[2000px] w-full max-w-2xl mx-auto my-20">
      <motion.div
        className="relative bg-plate border border-white/10 rounded-xl p-8 shadow-2xl overflow-hidden transform-gpu"
        style={{ rotateX, rotateY, transformStyle: "preserve-3d" }}
      >
        {/* Holographic Sheen */}
        <div className="absolute inset-0 bg-linear-to-tr from-cyan/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500" />
        
        {/* Header (Mac Style) */}
        <div className="flex items-center gap-2 mb-6 opacity-50">
          <div className="w-3 h-3 rounded-full bg-red-500/20" />
          <div className="w-3 h-3 rounded-full bg-yellow-500/20" />
          <div className="w-3 h-3 rounded-full bg-green-500/20" />
        </div>

        {/* Code Content */}
        <code className="block font-mono text-sm text-ghost/70 leading-relaxed">
          <span className="text-purple-400">func</span> <span className="text-yellow-200">initGhostKernel</span>() {'{'} <br/>
          &nbsp;&nbsp;<span className="text-ghost/40">// Initialize local reasoning engine</span> <br/>
          &nbsp;&nbsp;engine := <span className="text-cyan">gemini</span>.<span className="text-blue-400">NewLocalStream</span>(ModelV2) <br/>
          &nbsp;&nbsp;body := <span className="text-cyan">rho</span>.<span className="text-blue-400">ConnectPeripheral</span>() <br/>
          <br/>
          &nbsp;&nbsp;<span className="text-ghost/40">// Safety Gating</span> <br/>
          &nbsp;&nbsp;<span className="text-purple-400">if</span> !conscience.<span className="text-blue-400">AuditAction</span>(body) {'{'} <br/>
          &nbsp;&nbsp;&nbsp;&nbsp;return <span className="text-red-400">ErrUnsafeAction</span> <br/>
          &nbsp;&nbsp;{'}'} <br/>
          <br/>
          &nbsp;&nbsp;fmt.<span className="text-blue-400">Println</span>(<span className="text-green-300">"SYSTEM_ONLINE"</span>) <br/>
          {'}'}
        </code>

        {/* Floating Tag */}
        <motion.div 
            className="absolute -right-4 top-10 bg-cyan text-void text-[10px] font-bold px-2 py-1 rotate-90 translate-z-[50px] shadow-lg rounded"
            style={{ translateZ: 50 }}
        >
            KERNEL_MODE
        </motion.div>

      </motion.div>
    </div>
  );
};
