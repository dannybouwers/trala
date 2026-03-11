import fs from 'fs';
import path from 'path';

// Navigate from website/ to project root, then to docs/
const src = path.resolve('../docs');
const dest = path.resolve('./src/content/docs');

if (!fs.existsSync(dest)) {
  fs.mkdirSync(dest, { recursive: true });
}

const files = fs.readdirSync(src).filter(f => f.endsWith('.md'));
files.forEach(f => {
  fs.copyFileSync(path.join(src, f), path.join(dest, f));
  console.log(`Copied ${f} to ${dest}`);
});

console.log(`Copied ${files.length} docs files`);
