import fs from 'node:fs';
import path from 'node:path';

const root = process.cwd();
const ignored = new Set(['node_modules', '.git', '.expo', '.store-build', 'dist', 'dist-cloudflare', 'coverage', 'dist-web', '__tests__', 'test', 'scripts']);
const forbiddenNative = new Set(['Alert', 'ActivityIndicator', 'Dimensions', 'Pressable', 'SafeAreaView', 'ScrollView', 'Text', 'TextInput', 'TouchableOpacity']);
const failures = [];

function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (ignored.has(entry.name) || entry.name.startsWith('.')) continue;
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      walk(absolute);
      continue;
    }
    if (entry.name === 'app.config.ts' || !/\.(ts|tsx|js|jsx)$/.test(entry.name) || /\.(test|spec)\.[jt]sx?$/.test(entry.name)) continue;
    const relative = path.relative(root, absolute);
    if (relative.startsWith(path.join('src', 'ui'))) continue;
    const source = fs.readFileSync(absolute, 'utf8');
    const withoutComments = source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/.*$/gm, '');

    if (/Dimensions\.get\s*\(/.test(withoutComments)) failures.push(`${relative}: use useWindowDimensions/useResponsiveLayout instead of Dimensions.get`);
    if (/#[0-9a-f]{3,8}\b/i.test(withoutComments)) failures.push(`${relative}: move hard-coded colors to semantic UI tokens`);
    if (/(?:color|backgroundColor|borderColor|shadowColor)\s*:\s*['\"](?:white|black|red|green|blue|gray|grey|transparent)['\"]/i.test(withoutComments)) failures.push(`${relative}: move named colors to semantic UI tokens`);

    const importPattern = /import\s*\{([^}]*)\}\s*from\s*['"]react-native['"]/g;
    for (const match of withoutComments.matchAll(importPattern)) {
      const imports = match[1].split(',').map(value => value.trim()).filter(Boolean);
      const blocked = imports.filter(value => {
        const [name, alias] = value.split(/\s+as\s+/);
        return forbiddenNative.has(name) && !(name === 'ScrollView' && alias === 'NativeScrollView');
      });
      if (blocked.length) failures.push(`${relative}: replace native ${blocked.join(', ')} with Beeloy UI primitives`);
    }
  }
}

walk(root);
for (const required of ['src/ui/provider.tsx', 'src/ui/responsive.ts', 'src/ui/tokens.ts']) {
  if (!fs.existsSync(path.join(root, required))) failures.push(`${required}: required UI foundation file is missing`);
}

if (failures.length) {
  console.error(`UI guideline check failed (${failures.length}):\n${failures.map(value => `- ${value}`).join('\n')}`);
  process.exit(1);
}
console.log('UI guideline static check passed');
