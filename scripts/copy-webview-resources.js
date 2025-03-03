const fs = require('fs');
const path = require('path');

// Create output directory if it doesn't exist
const outWebviewDir = path.join(__dirname, '..', 'out', 'webview');
if (!fs.existsSync(outWebviewDir)) {
    fs.mkdirSync(outWebviewDir, { recursive: true });
}

// Copy webview HTML file
const srcHtmlPath = path.join(__dirname, '..', 'src', 'webview', 'canvas.html');
const destHtmlPath = path.join(outWebviewDir, 'canvas.html');

try {
    fs.copyFileSync(srcHtmlPath, destHtmlPath);
    console.log('Successfully copied webview resources to output directory');
} catch (err) {
    console.error('Error copying webview resources:', err);
    process.exit(1);
}