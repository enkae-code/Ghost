import { motion } from 'framer-motion';
import { InteractionDemo } from '../components/ui/InteractionDemo';

export const GhostProtocol = () => {
  return (
    <section className="relative py-40 flex flex-col items-center justify-center bg-void overflow-hidden min-h-[90vh]">
      {/* Cinematic Grain / Texture */}
      <div className="absolute inset-0 opacity-[0.03] pointer-events-none mix-blend-overlay" />
      
      {/* Background Radial Glow */}
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[1000px] h-[1000px] bg-cyan/5 blur-[150px] rounded-full pointer-events-none" />

      <motion.div 
        className="relative z-10 w-full max-w-6xl px-8 flex flex-col items-center"
        initial={{ opacity: 0, y: 40 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 1, ease: [0.22, 1, 0.36, 1] }}
      >
        <div className="text-center mb-20">
          <h2 className="text-[10px] font-mono text-cyan uppercase tracking-[0.6em] mb-6">The Sovereign Protocol</h2>
          <div className="text-5xl md:text-6xl font-bold text-white tracking-tighter leading-tight">
            Speak to the machine. <br />
            <span className="text-white/20">Let it execute your intent.</span>
          </div>
        </div>

        <div className="w-full max-w-4xl mx-auto">
          <InteractionDemo />
        </div>

        <div className="mt-24 grid grid-cols-1 md:grid-cols-3 gap-12 border-t border-white/5 pt-16 w-full">
            {[
                { title: "Neural Synthesis", desc: "Ghost translates high-level human intent into optimized code signatures instantly." },
                { title: "Spectral Execution", desc: "Operations run in isolated, high-security environments with zero latency." },
                { title: "Sovereign Control", desc: "You own the keys. Ghost owns the execution. Absolute privacy by design." }
            ].map((feature, idx) => (
                <motion.div 
                    key={idx}
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.3 + idx * 0.1, duration: 0.8 }}
                    className="space-y-4"
                >
                    <div className="text-[10px] font-bold text-white uppercase tracking-widest">{feature.title}</div>
                    <p className="text-[11px] text-white/30 leading-relaxed font-mono">
                        {feature.desc}
                    </p>
                </motion.div>
            ))}
        </div>
      </motion.div>
    </section>
  );
};
