import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';

const CODE_SNIPPET = [
  "async function scanInfrastructure(target) {",
  "  const connections = await bridge.fetch(target);",
  "  return connections.filter(c => c.active);",
  "}"
];

const SUGGESTED_CODE = [
  "async function scanInfrastructure(target) {",
  "  // ghost: optimized with parallel spectral pooling",
  "  const connections = await bridge.fetch(target, { mode: 'spectral' });",
  "  return connections.map(c => ({...c, integrity: 1.0 }));",
  "}"
];

export const InteractionDemo = () => {
  const [step, setStep] = useState<'prompt' | 'suggesting' | 'diff' | 'complete'>('prompt');
  const [promptText, setPromptText] = useState("");
  const targetPrompt = "Ghost, optimize this scan for spectral integrity";

  useEffect(() => {
    const timer = setTimeout(() => {
      if (step === 'prompt') {
        if (promptText.length < targetPrompt.length) {
          const charTimer = setTimeout(() => {
            setPromptText(targetPrompt.slice(0, promptText.length + 1));
          }, 40);
          return () => clearTimeout(charTimer);
        } else {
          setTimeout(() => setStep('suggesting'), 800);
        }
      }
    }, 1000);
    return () => clearTimeout(timer);
  }, [step, promptText]);

  useEffect(() => {
    if (step === 'suggesting') {
      setTimeout(() => setStep('diff'), 2000);
    }
    if (step === 'diff') {
      setTimeout(() => setStep('complete'), 1500);
    }
  }, [step]);

  return (
    <div className="flex flex-col items-center justify-center w-full max-w-4xl mx-auto py-12">
      {/* Editor Frame */}
      <div className="w-full bg-[#0B0F1A] rounded-xl border border-white/10 shadow-[0_20px_50px_rgba(0,0,0,0.5)] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 bg-white/5 border-b border-white/5">
          <div className="flex gap-1.5 font-mono text-[9px] text-white/20 uppercase tracking-widest">
            <span className="w-2.5 h-2.5 rounded-full bg-red-500/20" />
            <span className="w-2.5 h-2.5 rounded-full bg-yellow-500/20" />
            <span className="w-2.5 h-2.5 rounded-full bg-green-500/20" />
            <span className="ml-3">Ghost_Editor :: scan.js</span>
          </div>
          <div className="text-[9px] font-mono text-cyan/40">GHOST_ACTIVE</div>
        </div>

        {/* Content Area — single flow, no overlapping layers */}
        <div className="relative p-6 font-mono text-sm leading-relaxed overflow-hidden min-h-[220px]">
          {/* Prompt / Diff: show base code only */}
          {(step === 'prompt' || step === 'diff') && (
            <div className="space-y-2">
              {CODE_SNIPPET.map((line, i) => (
                <div key={i} className="flex gap-3 items-baseline">
                  <span className="w-6 shrink-0 text-right text-white/20 select-none tabular-nums">{i + 1}</span>
                  <span className={step === 'diff' && (i === 1 || i === 2) ? "text-white/30 line-through decoration-red-500/50" : "text-white/70"}>
                    {line}
                  </span>
                </div>
              ))}
            </div>
          )}

          {/* Suggesting: base code then suggestion block below (no overlay) */}
          {step === 'suggesting' && (
            <>
              <div className="space-y-2 mb-6">
                {CODE_SNIPPET.map((line, i) => (
                  <div key={i} className="flex gap-3 items-baseline">
                    <span className="w-6 shrink-0 text-right text-white/20 select-none tabular-nums">{i + 1}</span>
                    <span className="text-white/70">{line}</span>
                  </div>
                ))}
              </div>
              <motion.div
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                className="space-y-2 pt-4 border-t border-white/10"
              >
                <div className="text-[10px] uppercase tracking-widest text-cyan/50 mb-2">Ghost suggestion</div>
                {SUGGESTED_CODE.map((line, i) => (
                  <div key={i} className="flex gap-3 items-baseline">
                    <span className="w-6 shrink-0 text-right text-cyan/20 select-none tabular-nums">{i + 1}</span>
                    <span className="text-cyan/80 italic">{line}</span>
                  </div>
                ))}
              </motion.div>
            </>
          )}

          {/* Complete: only final code, no overlap */}
          {step === 'complete' && (
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              className="space-y-2"
            >
              {SUGGESTED_CODE.map((line, i) => {
                const isNew = i === 1 || i === 2 || i === 3;
                return (
                  <motion.div
                    key={i}
                    initial={isNew ? { backgroundColor: "rgba(0,240,255,0)" } : {}}
                    animate={isNew ? { backgroundColor: ["rgba(0,240,255,0.08)", "rgba(0,240,255,0)"] } : {}}
                    transition={{ duration: 0.6 }}
                    className="flex gap-3 items-baseline px-2 -mx-2 py-0.5 rounded"
                  >
                    <span className="w-6 shrink-0 text-right text-white/20 select-none tabular-nums">{i + 1}</span>
                    <span className={isNew ? "text-cyan" : "text-white/70"}>{line}</span>
                  </motion.div>
                );
              })}
            </motion.div>
          )}
        </div>

        {/* AI Command Bar (Prominent/Centered) */}
        <div className="p-4 bg-white/[0.02] border-t border-white/5">
          <div className="relative group">
            <div className="absolute inset-0 bg-cyan/5 blur-xl group-focus-within:bg-cyan/10 transition-colors" />
            <div className="relative flex items-center gap-3 px-4 py-3 bg-black/40 border border-white/10 rounded-lg group-focus-within:border-cyan/30 transition-all">
                <span className="text-[10px] font-bold text-cyan">⌘ K</span>
                <span className="text-white/80">{promptText}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Narrative Footer — Hollow Prism */}
      <footer className="mt-8 text-center space-y-3">
        <div className="text-[10px] font-mono text-white/20 uppercase tracking-[0.4em]">Integrated Intelligence</div>
        <p className="text-white/40 text-sm max-w-sm mx-auto leading-relaxed">
          Ghost observes your workflow and manifests architectural improvements in real-time.
        </p>
        <div className="pt-2 border-t border-white/5 text-[10px] font-mono text-white/15 uppercase tracking-[0.3em]">
          Ghost · Hollow Prism · Sovereign Infrastructure
        </div>
      </footer>
    </div>
  );
};
