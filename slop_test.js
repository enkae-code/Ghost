
// SLOP TEST FILE for Jules Sentinel
// This file contains intentional "slop" to verify the Auditor detects it.

const unusedVariable = "I am not used";

function testSlop() {
    console.log("Triggering Jules Auditor...");
    let x = 10;
    // Vague variable name
    let dataFromSomewhere = {
        val: true
    };
    return true;
}

testSlop();
