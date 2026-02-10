export const ProductFeatures = () => {
    return (
        <section className="relative z-10 py-32 px-6 max-w-4xl mx-auto">
             <div className="grid grid-cols-1 md:grid-cols-2 gap-16">
                
                {/* Column 1 */}
                <div className="space-y-16">
                    <div>
                        <span className="text-cyan font-mono text-xs tracking-widest uppercase mb-4 block">[ 01 ]</span>
                        <h3 className="text-2xl font-bold text-ghost mb-4">Local Intelligence</h3>
                        <p className="text-ghost/60 leading-relaxed">
                            Native support for Ollama and LM Studio. Run Llama 3, Mistral, or Gemma directly on your hardware. No cloud, no API keys, no latency.
                        </p>
                    </div>

                    <div>
                         <span className="text-cyan font-mono text-xs tracking-widest uppercase mb-4 block">[ 02 ]</span>
                        <h3 className="text-2xl font-bold text-ghost mb-4">Desktop Control</h3>
                        <p className="text-ghost/60 leading-relaxed">
                            Deep system integration. Ghost can execute shell commands, manipulate files, and control applications. It is not a chatbot; it is an operator.
                        </p>
                    </div>
                </div>

                {/* Column 2 */}
                <div className="space-y-16 md:mt-32">
                     <div>
                         <span className="text-cyan font-mono text-xs tracking-widest uppercase mb-4 block">[ 03 ]</span>
                        <h3 className="text-2xl font-bold text-ghost mb-4">Voice Interface</h3>
                        <p className="text-ghost/60 leading-relaxed">
                            Real-time speech-to-speech with near-zero latency. Hands-free execution loop for when you need to be away from the keyboard.
                        </p>
                    </div>

                    <div>
                         <span className="text-cyan font-mono text-xs tracking-widest uppercase mb-4 block">[ 04 ]</span>
                        <h3 className="text-2xl font-bold text-ghost mb-4">Zero Telemetry</h3>
                        <p className="text-ghost/60 leading-relaxed">
                            Your data never leaves your machine. 100% local processing. 
                            <span className="text-ghost block mt-2">Audit the source code on GitHub.</span>
                        </p>
                    </div>
                </div>

             </div>
        </section>
    );
};
