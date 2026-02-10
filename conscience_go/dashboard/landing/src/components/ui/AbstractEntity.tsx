import { motion } from "framer-motion";

export const AbstractEntity = () => {
    return (
        <div className="relative w-64 h-64 flex items-center justify-center">
            {/* Ghost Logo Reference */}
            <motion.img 
                src="/ghost-logo-transparent.png" 
                alt="Ghost Entity" 
                className="w-full h-full object-contain drop-shadow-[0_0_15px_rgba(34,211,238,0.3)]"
                initial={{ opacity: 0, scale: 0.9 }}
                animate={{ 
                    opacity: 1, 
                    scale: [1, 1.05, 1],
                    filter: [
                        "drop-shadow(0 0 15px rgba(34,211,238,0.3))",
                        "drop-shadow(0 0 25px rgba(34,211,238,0.5))",
                        "drop-shadow(0 0 15px rgba(34,211,238,0.3))"
                    ]
                }}
                transition={{ 
                    duration: 4, 
                    repeat: Infinity,
                    ease: "easeInOut" 
                }}
            />
        </div>
    );
};
