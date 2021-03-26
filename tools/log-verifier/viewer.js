#!/usr/local/bin/node
// USAGE:  ./viewer.js <FILE>
// Can then use n, right or down to move to next line
// use p, left or up to go to previous line
// use q to quit

const { Console } = require("console");
const fs = require("fs"),
      readline = require("readline");
readline.emitKeypressEvents(process.stdin);
process.stdin.setRawMode(true);
console.log(process.argv);
if (process.argv.length <= 2) {
    console.log("No file provided")
} else {
    let file = process.argv.slice(2)[0];
    var lines = []
    const reader = readline.createInterface({
        input: fs.createReadStream(file),
        crlfDelay: Infinity
    });
    reader.on('line', (line) => {
        lines.push(line);
    });
    reader.on('close', () => {
        let lineNumber = 1;
        processLine(file, lines[lineNumber -1], lineNumber);
        process.stdin.on("keypress", (str, key) => {
            if (str == "n" || key.name == "right" || key.name == "down") {
                lineNumber++;
            } else if (str == "q") {
                process.exit(0);
            } else if (str == "p" || key.name == "left" || key.name == "up") {
                lineNumber--;
            } 
            if (lineNumber > lines.length + 1) {
                console.log(`End of ${file}`);
                process.exit(0);
            }
            processLine(file, lines[lineNumber -1], lineNumber);
        })
    });
    function processLine(fileName, line, lineNumber) {
            console.log(`FileName: ${fileName}\tLine Number: ${lineNumber}`);
            if (line && line.length > 0) {
                try {
                    var jsonLine = line.substring(line.indexOf(":") + 1);
                    console.log(JSON.stringify(JSON.parse(jsonLine), null, 4));
                } catch {
                    console.error(`Error occurred during standard parsing on line ${lineNumber}`);
                    console.error(line);
                }
            } else {
                console.log(`Line ${lineNumber} is an empty line.`);
            }
            console.log("Press 'n' for next line, 'p' for previous line, or 'q' to exit");
        return;
    }
}