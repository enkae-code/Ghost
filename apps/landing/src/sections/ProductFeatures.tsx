import { useRef } from 'react';
import { gsap } from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';
import { useGSAP } from '@gsap/react';

gsap.registerPlugin(ScrollTrigger);

export const ProductFeatures = () => {
    const sectionRef = useRef<HTMLElement>(null);
    const cardsRef = useRef<(HTMLDivElement | null)[]>([]);

    useGSAP(() => {
        const cards = cardsRef.current.filter(Boolean); // Filter nulls
        
        gsap.fromTo(cards, 
            { y: 50, opacity: 0 },
            {
                y: 0,
                opacity: 1,
                duration: 0.8,
                stagger: 0.1,
                ease: "power3.out",
                scrollTrigger: {
                    trigger: sectionRef.current,
                    start: "top 80%",
                    toggleActions: "play none none reverse"
                }
            }
        );
    }, { scope: sectionRef });

    const features = [
        { id: "01", title: "Local Intelligence", desc: "Native support for Ollama and LM Studio. Run Llama 3, Mistral, or Gemma directly on your hardware. No cloud, no API keys, no latency." },
        { id: "02", title: "Desktop Control", desc: "Deep system integration. Ghost can execute shell commands, manipulate files, and control applications. It is not a chatbot; it is an operator." },
        { id: "03", title: "Voice Interface", desc: "Real-time speech-to-speech with near-zero latency. Hands-free execution loop for when you need to be away from the keyboard." },
        { id: "04", title: "Zero Telemetry", desc: "Your data never leaves your machine. 100% local processing.", extra: "Audit the source code on GitHub." }
    ];

    return (
        <section ref={sectionRef} className="relative z-10 py-32 px-6 max-w-4xl mx-auto">
             <div className="grid grid-cols-1 md:grid-cols-2 gap-8"> {/* Changed gap-16 to gap-8 for tighter cards */}
                {features.map((feature, index) => (
                    <div 
                        key={feature.id}
                        ref={el => { cardsRef.current[index] = el }}
                        className="glass-panel rounded-2xl p-8 hover:border-cyan/50 transition-colors duration-300 group relative overflow-hidden"
                    >
                        {/* Glow Blob */}
                        <div className="absolute -top-20 -right-20 w-40 h-40 bg-cyan/20 blur-[80px] rounded-full opacity-0 group-hover:opacity-100 transition-opacity duration-500 pointer-events-none" />

                        <span className="text-cyan font-mono text-xs tracking-widest uppercase mb-4 block">[ {feature.id} ]</span>
                        <h3 className="text-2xl font-bold text-ghost mb-4 group-hover:text-white transition-colors">{feature.title}</h3>
                        <p className="text-ghost/60 leading-relaxed font-light">
                            {feature.desc}
                            {feature.extra && <span className="text-ghost block mt-2 font-medium">{feature.extra}</span>}
                        </p>
                    </div>
                ))}
             </div>
        </section>
    );
};
