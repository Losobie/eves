// Regenerate fixtures using an unpacked @truebrain/blue-marshal 1.0.1 package:
// node internal/bluemarshal/testdata/generate.mjs <package-directory>
// This is a development-only oracle; eves has no Node/WASM runtime dependency.
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { pathToFileURL } from 'node:url';

const packageDir = resolve(process.argv[2]);
const bindings = await import(pathToFileURL(join(packageDir, 'blue_marshal_wasm_bg.js')));
const module = new WebAssembly.Module(readFileSync(join(packageDir, 'blue_marshal_wasm_bg.wasm')));
const imports = Object.fromEntries(WebAssembly.Module.imports(module).map(({ module }) => [module, bindings]));
const instance = new WebAssembly.Instance(module, imports);
bindings.__wbg_set_wasm(instance.exports);
instance.exports.__wbindgen_start();

const tuple = (...items) => ({ tuple: items });
const probe = (x, y, z) => tuple(tuple(x, y, z), 37399467675);
const root = {
  'bytes:ui': {
    'bytes:probescanning.customFormations': tuple('long:134280801871504062', {
      'int:7': tuple('utf8:Drifter: α 🚀', [probe(250000, 0, 0), probe(-250000, 0, 0)]),
      'int:2': tuple('bytes:Pinpoint', [probe(0, 500000, 0)]),
      'int:-4': tuple('bytes:tempFormation', []),
    }),
    'bytes:probescanning.selectedFormationID': tuple('long:134280801871504062', 7),
    'bytes:unrelated': tuple('long:9999999999999999999999999999', {
      'bytes:bool': true,
      'bytes:float': 1.5,
      'bytes:negative': -40000,
      'bytes:long': 'long:-99999999999999999999999999',
      'bytes:instance': { instance: { class: 'util.KeyVal', state: { 'bytes:test': null } } },
    }),
  },
};
for (const [name, value] of Object.entries({ formations: root, empty: { 'bytes:ui': {} } })) {
  const bytes = bindings.encode_from_json(JSON.stringify(value));
  // Ensure the reference decoder accepts the exact fixture before saving it.
  const decoded = bindings.decode_to_json(bytes);
  writeFileSync(new URL(`${name}.dat`, import.meta.url), bytes);
  writeFileSync(new URL(`${name}.json`, import.meta.url), decoded + '\n');
}
