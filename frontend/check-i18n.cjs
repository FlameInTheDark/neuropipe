const fs = require("fs");
const ts = require("typescript");
const outDir = "node_modules/.cache/i18n-check";
fs.rmSync(outDir, { recursive: true, force: true });
fs.mkdirSync(outDir, { recursive: true });
const program = ts.createProgram(["src/i18n/en.ts", "src/i18n/de.ts", "src/i18n/fr.ts", "src/i18n/ru.ts"], {
  target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS, outDir, skipLibCheck: true, noEmitOnError: false,
});
program.emit();
const en = require("./" + outDir + "/en.js").en;
const langs = { de: require("./" + outDir + "/de.js").de, fr: require("./" + outDir + "/fr.js").fr, ru: require("./" + outDir + "/ru.js").ru };

const PLURAL = ["one", "few", "many", "other", "zero", "two"];
function lookup(obj, dotted) {
  let cur = obj;
  for (const p of dotted.split(".")) {
    if (cur == null || typeof cur !== "object") return undefined;
    cur = cur[p];
  }
  return typeof cur === "string" ? cur : undefined;
}
/** exact key, or any plural-suffix variant (code calls t("status.nodes", {count})) */
function hasKey(obj, dotted) {
  if (lookup(obj, dotted) !== undefined) return true;
  return PLURAL.some((sfx) => lookup(obj, dotted + "_" + sfx) !== undefined);
}

const files = [];
function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = dir + "/" + e.name;
    if (e.isDirectory()) walk(p);
    else if (/\.tsx?$/.test(e.name) && !p.replace(/\\/g, "/").includes("i18n/")) files.push(p);
  }
}
walk("src");
const used = new Map();
for (const f of files) {
  const src = fs.readFileSync(f, "utf8");
  for (const m of src.matchAll(/\bt\(\s*"([^"]+)"/g)) if (!used.has(m[1])) used.set(m[1], f);
  for (const m of src.matchAll(/i18n\.t\(\s*"([^"]+)"/g)) if (!used.has(m[1])) used.set(m[1], f);
  for (const m of src.matchAll(/\bt\(\s*`([^`]+)`/g)) {
    const raw = m[1];
    const base = raw.split("${")[0].replace(/\.$/, "");
    const parts = base.split(".");
    let cur = en;
    for (const p of parts) cur = cur && typeof cur === "object" ? cur[p] : undefined;
    if (cur && typeof cur === "object" && Object.keys(cur).length > 0) {
      for (const k of Object.keys(cur)) if (!used.has(base + "." + k)) used.set(base + "." + k, f);
    } else if (raw.includes("${")) {
      // flat prefix keys, e.g. pins.type_${x} — collect matching flat keys one level up
      const parentPath = base.split(".").slice(0, -1).join(".");
      const prefix = base.split(".").pop();
      let parent = en;
      for (const p of parentPath.split(".")) parent = parent && typeof parent === "object" ? parent[p] : undefined;
      if (parent && typeof parent === "object") {
        for (const k of Object.keys(parent)) if (k.startsWith(prefix)) if (!used.has(parentPath + "." + k)) used.set(parentPath + "." + k, f);
      } else {
        console.log("TEMPLATE-UNRESOLVED:", raw, "(" + f + ")");
      }
    }
  }
}

let missingEn = 0;
for (const key of [...used.keys()].sort()) {
  if (!hasKey(en, key)) { console.log("MISSING en:", key, "(" + used.get(key) + ")"); missingEn++; }
}
console.log("---");
let missOther = 0;
for (const [lang, cat] of Object.entries(langs)) {
  for (const key of [...used.keys()].sort()) {
    if (hasKey(en, key) && !hasKey(cat, key)) { console.log(`MISSING ${lang}:`, key); missOther++; }
  }
}
// reverse check: catalog keys never referenced in code (dead translations)
function flatten(obj, prefix, out) {
  for (const [k, v] of Object.entries(obj)) {
    if (typeof v === "object") flatten(v, prefix + k + ".", out);
    else {
      const base = prefix + k.replace(/_(one|few|many|other|zero|two)$/, "");
      out.add(base);
    }
  }
}
const enFlat = new Set();
flatten(en, "", enFlat);
const dead = [...enFlat].filter((k) => !used.has(k)).sort();
for (const d of dead) console.log("UNUSED en:", d);
console.log(`checked=${used.size} missing_en=${missingEn} missing_other=${missOther} unused_en=${dead.length}`);
