import { Logo } from '../ui/Logo';
import { DecryptedText } from '../ui/DecryptedText';
import { motion } from 'framer-motion';

export const Navbar = () => {
  return (
    <motion.nav 
      initial={{ y: -100, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      transition={{ duration: 1, ease: [0.22, 1, 0.36, 1] }}
      className="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-8 py-5 pointer-events-none"
    >
      <div className="flex items-center gap-6 pointer-events-auto">
        <Logo className="w-8 h-8 text-cyan hover:scale-110 transition-transform duration-500 cursor-pointer" />
        <div className="flex flex-col">
          <DecryptedText 
            text="GHOST" 
            speed={40} 
            className="text-xs font-bold tracking-[0.4em] text-white/90 font-mono"
          />
          <div className="h-px w-full bg-linear-to-r from-cyan/50 to-transparent mt-1" />
        </div>
      </div>

      <div className="hidden lg:flex items-center gap-10 text-[10px] font-mono text-white/40 uppercase tracking-[0.3em] pointer-events-auto">
        {['Features', 'Installation', 'Manifesto'].map((item) => (
          <a 
            key={item} 
            href={`#${item.toLowerCase()}`} 
            className="hover:text-white transition-colors duration-500 relative group"
          >
            {item}
            <span className="absolute -bottom-1 left-0 w-0 h-px bg-cyan group-hover:w-full transition-all duration-500" />
          </a>
        ))}
      </div>
      
      <div className="flex items-center gap-6 pointer-events-auto">
        <button className="hidden sm:block text-[10px] font-bold text-white/60 hover:text-white border border-white/10 hover:border-white/20 px-5 py-2 transition-all uppercase tracking-widest">
          Connect
        </button>
      </div>
    </motion.nav>
  );
};
