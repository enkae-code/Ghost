import { Logo } from '../ui/Logo';
import { DecryptedText } from '../ui/DecryptedText';
import { motion } from 'framer-motion';

export const Navbar = () => {
  return (
    <motion.nav 
      initial={{ y: -100 }}
      animate={{ y: 0 }}
      transition={{ duration: 0.8, ease: "circOut" }}
      className="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 glass-panel border-b border-white/5"
    >
      <div className="flex items-center gap-4">
        <Logo className="w-8 h-8 text-cyan" />
        <DecryptedText 
          text="GHOST" 
          speed={30} 
          className="text-lg font-bold tracking-widest text-white font-mono"
        />
      </div>

      <div className="hidden md:flex items-center gap-8 text-xs font-mono text-ghost/60 uppercase tracking-widest">
        {['Features', 'Installation', 'Manifesto'].map((item) => (
          <a key={item} href={`#${item.toLowerCase()}`} className="hover:text-cyan transition-colors duration-300">
            {item}
          </a>
        ))}
      </div>
      
      <button className="px-4 py-2 text-xs font-bold bg-white text-black hover:bg-cyan transition-colors uppercase tracking-wider">
        Get Started
      </button>
    </motion.nav>
  );
};
