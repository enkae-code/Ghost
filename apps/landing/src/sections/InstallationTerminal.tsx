import { Copy } from 'lucide-react';
import { useRef, useState } from 'react';
import { gsap } from 'gsap';
import { TextPlugin } from 'gsap/TextPlugin';
import { useGSAP } from '@gsap/react';

gsap.registerPlugin(TextPlugin);

export const InstallationTerminal = () => {
    const [copied, setCopied] = useState(false);
    const commandText = "curl -sL https://ghost.sh/install | bash";
    const textRef = useRef<HTMLElement>(null);
    const cursorRef = useRef<HTMLElement>(null);

    useGSAP(() => {
        const tl = gsap.timeline({ repeat: -1, repeatDelay: 2 });
        
        // Typewriter effect
        tl.to(textRef.current, {
            duration: 3,
            text: commandText,
            ease: "none",
        });

        // Blinking cursor (runs independently)
        gsap.to(cursorRef.current, {
            opacity: 0,
            duration: 0.5,
            repeat: -1,
            yoyo: true,
            ease: "steps(1)"
        });

    }, []);

    const handleCopy = () => {
        navigator.clipboard.writeText(commandText);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <section className="py-24 px-6 max-w-4xl mx-auto text-center">
            <div 
                className="relative glass-panel rounded-xl px-8 py-6 flex items-center justify-center gap-6 hover:border-cyan/50 transition-colors cursor-pointer group" 
                onClick={handleCopy}
            >
                <div className="absolute top-0 left-0 w-full h-full bg-slate-900/50 rounded-xl -z-10" />
                
                <span className="text-cyan font-mono text-xl mr-auto">$</span>
                <div className="flex items-center">
                    <code ref={textRef} className="text-ghost font-mono text-xl tracking-wide min-h-[1.75rem]"></code>
                    <span ref={cursorRef} className="text-cyan font-mono text-xl ml-1">_</span>
                </div>
                
                <div className="ml-auto text-ghost/30 group-hover:text-ghost/60 transition-colors">
                     {copied ? <span className="text-cyan text-xs tracking-widest uppercase">Copied</span> : <Copy size={18} />}
                </div>
            </div>
            
            <p className="mt-6 text-ghost/40 text-sm font-light">
                Supports macOS (Silicon/Intel), Linux, and Windows (WSL2).
            </p>
        </section>
    );
};
