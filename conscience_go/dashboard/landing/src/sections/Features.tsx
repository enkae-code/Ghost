import { BentoGrid } from "../components/ui/BentoGrid";
import { BentoCard } from "../components/ui/BentoCard";
import { Shield, Cpu, Activity, Lock, Terminal, Zap } from "lucide-react";

export const Features = () => {
    return (
        <section className="px-4 py-20 max-w-7xl mx-auto relative z-10">
            <BentoGrid>
                {/* Tri-Brain Concept */}
                <BentoCard colSpan={8} title="Tri-Brain Architecture" subtitle="Biologically inspired cognitive separation.">
                    <div className="flex gap-4 mt-4 text-ghost/80">
                         <div className="flex-1 p-4 bg-void/50 rounded-lg border border-white/5">
                            <div className="flex items-center gap-2 mb-2 text-cyan">
                                <Cpu size={20} />
                                <h4 className="font-bold">Brain</h4>
                            </div>
                            <p className="text-sm">High-level reasoning and planning (Gemini 2.0).</p>
                         </div>
                         <div className="flex-1 p-4 bg-void/50 rounded-lg border border-white/5">
                            <div className="flex items-center gap-2 mb-2 text-purple-400">
                                <Activity size={20} />
                                <h4 className="font-bold">Body</h4>
                            </div>
                            <p className="text-sm">Execution and tool use (Rho-1).</p>
                         </div>
                         <div className="flex-1 p-4 bg-void/50 rounded-lg border border-white/5">
                            <div className="flex items-center gap-2 mb-2 text-green-400">
                                <Shield size={20} />
                                <h4 className="font-bold">Conscience</h4>
                            </div>
                            <p className="text-sm">Kernel-level safety gating.</p>
                         </div>
                    </div>
                </BentoCard>

                {/* Local First */}
                 <BentoCard colSpan={4} title="Sovereign" subtitle="Local-first by default.">
                    <div className="h-full flex flex-col justify-center items-center text-center p-4">
                        <Lock className="w-16 h-16 text-cyan mb-4 opacity-80" />
                        <p className="text-sm text-ghost/70">
                            Zero telemetry. Your data never leaves the `localhost` airgap unless explicitly authorized.
                        </p>
                    </div>
                </BentoCard>

                {/* Integration */}
                <BentoCard colSpan={4} title="Universal Scribe" subtitle="Edit any file.">
                    <div className="bg-surface/50 p-4 rounded-lg font-mono text-xs text-green-400 border border-white/5">
                        <p>$ edit src/main.go</p>
                        <p className="text-ghost/50">{`> Opening buffer...`}</p>
                        <p className="text-ghost/50">{`> Context loaded.`}</p>
                        <p className="animate-pulse">_</p>
                    </div>
                </BentoCard>

                {/* Speed */}
                <BentoCard colSpan={4} title="0ms Latency" subtitle="Optimized for consumer hardware.">
                     <div className="relative h-24 overflow-hidden rounded-lg bg-void/50 border border-white/5 flex items-center justify-center">
                        <div className="absolute inset-0 bg-gradient-to-r from-transparent via-cyan/20 to-transparent w-[200%] animate-shimmer" />
                        <Zap className="text-cyan fill-cyan relative z-10" />
                     </div>
                </BentoCard>

                {/* Terminal/CLI */}
                <BentoCard colSpan={4} title="Anti-Slop" subtitle="Precision tools only.">
                    <div className="flex items-center justify-center h-full text-ghost/30">
                        <Terminal size={48} />
                    </div>
                </BentoCard>

            </BentoGrid>
        </section>
    );
};
