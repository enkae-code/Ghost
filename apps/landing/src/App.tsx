import { SovereignHero } from "./sections/SovereignHero";
import { GhostProtocol } from "./sections/GhostProtocol";
import { ProductFeatures } from "./sections/ProductFeatures";
import { InstallationTerminal } from "./sections/InstallationTerminal";
import { CodeHologram } from "./components/premium/CodeHologram";

import { Navbar } from "./components/layout/Navbar";

function App() {
  return (
    <main className="min-h-screen bg-void text-ghost selection:bg-cyan/30 selection:text-cyan overflow-hidden font-sans">
       {/* Global Scanline Overlay */}
       <div className="fixed inset-0 pointer-events-none z-50 opacity-[0.02] bg-[url('https://grainy-gradients.vercel.app/noise.svg')] brightness-100 contrast-150 mix-blend-overlay"></div>
       
       <Navbar />

       {/* Main Content Sections */}
       <main>
           <SovereignHero />
           <GhostProtocol />
           <ProductFeatures />
           <InstallationTerminal />
       </main>
       
       {/* The holographic code block as a distinctive section separator */}
       <section className="relative z-10 px-6 pb-32">
          <div className="text-center mb-8">
              <span className="text-label text-cyan/50">Core Logic</span>
          </div>
          <CodeHologram />
       </section>

       <footer className="py-12 border-t border-white/5 bg-plate/30 text-center">
          <span className="text-slate-500 text-xs font-mono">© 2026 Hollow Prism™ LLC. All rights reserved.</span>
       </footer>
    </main>
  )
}

export default App
