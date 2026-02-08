async function loop() {
    try {
        const res = await fetch('/v1/system/state');
        const data = await res.json();
        
        // Update Connection Badge
        const badge = document.getElementById('conn-status');
        badge.innerText = "LIVE";
        badge.classList.add('live');
        
        // Update Focus Text
        if (data.activeFocus) {
            document.getElementById('active-focus').innerText = "WATCHING: " + data.activeFocus;
            
            const focusEl = document.getElementById('focus-data');
            focusEl.innerText = data.activeFocus;
            focusEl.classList.remove('empty');
        }

        // Update Ghost State (Visual Feedback)
        const hero = document.getElementById('hero');
        // If state is 'THINKING' or 'PROCESSING' (Logic to be expanded)
        if (data.state === 'THINKING') {
            hero.classList.add('thinking');
        } else {
            hero.classList.remove('thinking');
        }

    } catch(e) {
        const badge = document.getElementById('conn-status');
        badge.innerText = "DISCONNECTED";
        badge.classList.remove('live');
    }
}

// 2Hz Poll Rate
setInterval(loop, 500);
loop();
