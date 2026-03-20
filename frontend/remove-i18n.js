const fs = require('fs');
const path = require('path');

const enJsonPath = path.join(__dirname, 'src/messages/en.json');
const en = JSON.parse(fs.readFileSync(enJsonPath, 'utf8'));

function walkSync(dir, filelist = []) {
  fs.readdirSync(dir).forEach(file => {
    const dirFile = path.join(dir, file);
    if (fs.statSync(dirFile).isDirectory()) {
      if (!dirFile.includes('node_modules') && !dirFile.includes('.next')) {
        filelist = walkSync(dirFile, filelist);
      }
    } else {
      if (dirFile.endsWith('.tsx') || dirFile.endsWith('.ts')) {
        filelist.push(dirFile);
      }
    }
  });
  return filelist;
}

const files = walkSync(path.join(__dirname, 'src'));

files.forEach(file => {
  let content = fs.readFileSync(file, 'utf8');
  let changed = false;

  // 1. Replace imports
  if (content.includes("from 'next-intl'")) {
    content = content.replace(/import\s*{[^}]*useTranslations[^}]*}\s*from\s*['"]next-intl['"];?/g, '');
    content = content.replace(/import\s*{[^}]*NextIntlClientProvider[^}]*}\s*from\s*['"]next-intl['"];?/g, '');
    changed = true;
  }
  if (content.includes("from 'next-intl/server'")) {
    content = content.replace(/import\s*{[^}]*getMessages[^}]*}\s*from\s*['"]next-intl\/server['"];?/g, '');
    changed = true;
  }
  
  if (content.includes("from '@/navigation'")) {
    content = content.replace(/import\s*\{\s*Link\s*\}\s*from\s*['"]@\/navigation['"];?/g, "import Link from 'next/link';");
    content = content.replace(/import\s*\{\s*usePathname,\s*useRouter\s*\}\s*from\s*['"]@\/navigation['"];?/g, "import { usePathname, useRouter } from 'next/navigation';");
    content = content.replace(/import\s*\{\s*useRouter,\s*usePathname\s*\}\s*from\s*['"]@\/navigation['"];?/g, "import { usePathname, useRouter } from 'next/navigation';");
    content = content.replace(/import\s*\{\s*useRouter\s*\}\s*from\s*['"]@\/navigation['"];?/g, "import { useRouter } from 'next/navigation';");
    changed = true;
  }

  // 2. Find useTranslations declarations
  const useTranslationRegex = /const\s+([a-zA-Z0-9_]+)\s*=\s*useTranslations\(['"]([^'"]+)['"]\);?/g;
  let match;
  const tMap = {}; // localVarName -> Namespace
  while ((match = useTranslationRegex.exec(content)) !== null) {
    tMap[match[1]] = match[2];
    changed = true;
  }

  // Remove the declarations
  content = content.replace(useTranslationRegex, '');

  // 3. Replace all t('key') or similar with actual text
  for (const [tVar, namespace] of Object.entries(tMap)) {
    // regex to catch: tVar('key')
    const callRegex = new RegExp(`${tVar}\\(['"]([^'"]+)['"]\\)`, 'g');
    content = content.replace(callRegex, (fullMatch, key) => {
      // Handle nested keys like "stats.total"
      let val = en[namespace];
      if (!val) return `"${key}"`; // fallback
      
      const parts = key.split('.');
      for (const p of parts) {
        if (val && val[p] !== undefined) {
          val = val[p];
        } else {
          val = key; // fallback
          break;
        }
      }
      
      if (typeof val === 'string') {
        // returning as a quoted string or template literal
        // Wait, what if it's `{t('key')}` inside TSX? It will be replaced with `{"Value"}` which is valid, or `{"Value"}`.
        // Actually `{t('key')}` -> `{"Value"}` is perfectly valid in JSX.
        // What if it's already inside a JS context? `message.success(t('key'))` -> `message.success("Value")`.
        // To handle escaping, we use JSON.stringify
        return JSON.stringify(val);
      }
      return JSON.stringify(key);
    });
  }

  if (changed) {
    fs.writeFileSync(file, content, 'utf8');
    console.log(`Updated ${file}`);
  }
});

console.log('Done!');
