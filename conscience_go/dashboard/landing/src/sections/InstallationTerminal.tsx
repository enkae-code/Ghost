import { Copy } from 'lucide-react';
import { useState } from 'react';

export const InstallationTerminal = () => {
    const [copied, setCopied] = useState(false);
    const command = "curl -sL https://ghost.sh/install | bash";

    const handleCopy = () => {
        navigator.clipboard.writeText(command);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    return (
        <section className="py-24 px-6 max-w-4xl mx-auto text-center">
            <div className="relative bg-black border border-white/10 px-8 py-6 flex items-center justify-center gap-6 hover:border-cyan/50 transition-colors cursor-pointer" onClick={handleCopy}>
                <span className="text-cyan font-mono text-xl mr-auto">$</span>
                <code className="text-ghost font-mono text-xl tracking-wide">{command}</code>
                <div className="ml-auto text-ghost/30">
                     {copied ? <span className="text-cyan text-xs tracking-widest uppercase">Copied</span> : <Copy size={18} />}
                </div>
            </div>
            
            <p className="mt-6 text-ghost/40 text-sm font-light">
                Supports macOS (Silicon/Intel), Linux, and Windows (WSL2).
            </p>
        </section>
    );
};
