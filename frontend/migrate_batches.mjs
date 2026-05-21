import fs from 'fs';
import path from 'path';

const APP_TSX_PATH = path.join(process.cwd(), 'src/App.tsx');
const HOME_PAGE_PATH = path.join(process.cwd(), 'src/pages/HomePage.tsx');

let appContent = fs.readFileSync(APP_TSX_PATH, 'utf-8');
let homeContent = fs.readFileSync(HOME_PAGE_PATH, 'utf-8');

// The items we want to extract
const statesToExtract = [
  "const [prompt, setPrompt] =",
  "const [size, setSize] =",
  "const [style, setStyle] =",
  "const [count, setCount] =",
  "const [quality, setQuality] =",
  "const [outputFormat, setOutputFormat] =",
  "const [customWidth, setCustomWidth] =",
  "const [customHeight, setCustomHeight] =",
  "const [referenceImageURL, setReferenceImageURL] =",
  "const [referenceImageName, setReferenceImageName] =",
  "const [referenceImageSource, setReferenceImageSource] =",
  "const [referenceImages, setReferenceImages] =",
  "const [referenceImagesDialogOpen, setReferenceImagesDialogOpen] =",
  "const [showNetworkImageInput, setShowNetworkImageInput] =",
  "const [referenceImageUploading, setReferenceImageUploading] =",
  "const [promptDragActive, setPromptDragActive] =",
  "const [generateConfirmOpen, setGenerateConfirmOpen] =",
  "const referenceImageInputRef ="
];

const funcsToExtract = [
  "const estimatedGenerationPointCost =",
  "function getReferenceImageExtraPointCost",
  "function handlePromptDrag",
  "function handlePromptDragLeave",
  "function handlePromptDrop",
  "function handlePromptPaste",
  "function handleClearPromptOptions",
  "async function handleLocalReferenceImageChange",
  "function handleNetworkReferenceImageInput",
  "async function handleAddNetworkReferenceImage",
  "function handleDeleteReferenceImage",
  "function handleOpenCreateTaskConfirm",
  "function handleCancelCreateTaskConfirm",
  "function handleConfirmCreateTask",
  "async function handleCreateTask"
];

let appLines = appContent.split(/\r?\n/);

function extractBlock(startMarker, isFunc = true) {
  const startIdx = appLines.findIndex(l => l.includes(startMarker));
  if (startIdx === -1) return null;
  
  if (!isFunc) {
      // it's a single line state
      const block = appLines[startIdx];
      appLines.splice(startIdx, 1);
      return block;
  }
  
  let endIdx = startIdx;
  let braceCount = 0;
  let foundBrace = false;
  for (let i = startIdx; i < appLines.length; i++) {
    const line = appLines[i];
    for (let j = 0; j < line.length; j++) {
      if (line[j] === '{') { braceCount++; foundBrace = true; }
      else if (line[j] === '}') { braceCount--; }
    }
    if (foundBrace && braceCount === 0) {
      endIdx = i;
      break;
    }
  }
  const block = appLines.slice(startIdx, endIdx + 1).join('\n');
  appLines.splice(startIdx, endIdx - startIdx + 1);
  return block;
}

const extractedStates = [];
for (const s of statesToExtract) {
  const code = extractBlock(s, false);
  if (code) extractedStates.push(code.trim());
}

const extractedFuncs = [];
for (const f of funcsToExtract) {
  const code = extractBlock(f, true);
  if (code) extractedFuncs.push(code.trim());
}

// Extract Dialogs from App.tsx rendering
let appText = appLines.join('\n');
const refDialogStart = appText.indexOf('{referenceImagesDialogOpen ? (');
if (refDialogStart !== -1) {
  const refDialogEnd = appText.indexOf(') : null}', refDialogStart) + 9;
  const refDialogCode = appText.substring(refDialogStart, refDialogEnd);
  appText = appText.substring(0, refDialogStart) + appText.substring(refDialogEnd);
  extractedFuncs.push("/* RENDERS */\n" + refDialogCode);
}

const confirmDialogStart = appText.indexOf('{generateConfirmOpen ? (');
if (confirmDialogStart !== -1) {
  const confirmDialogEnd = appText.indexOf(') : null}', confirmDialogStart) + 9;
  const confirmDialogCode = appText.substring(confirmDialogStart, confirmDialogEnd);
  appText = appText.substring(0, confirmDialogStart) + appText.substring(confirmDialogEnd);
  extractedFuncs.push(confirmDialogCode);
}

// Remove props from <HomePage /> in App.tsx
const propsToRemove = [
  "prompt=", "setPrompt=", "handlePromptDrag=", "handlePromptDragLeave=", "handlePromptDrop=", "handlePromptPaste=",
  "promptDragActive=", "referenceImageInputRef=", "referenceImageUploading=", "setShowNetworkImageInput=",
  "showNetworkImageInput=", "handleNetworkReferenceImageInput=", "referenceImages=", "handleDeleteReferenceImage=",
  "setReferenceImagesDialogOpen=", "referenceImagesDialogOpen=", "getReferenceImageExtraPointCost=", "size=", "setSize=",
  "customWidth=", "setCustomWidth=", "customHeight=", "setCustomHeight=", "style=", "setStyle=", "count=", "setCount=",
  "quality=", "setQuality=", "outputFormat=", "setOutputFormat=", "estimatedGenerationPointCost=",
  "handleOpenCreateTaskConfirm=", "handleClearPromptOptions=", "generateConfirmOpen=", "setGenerateConfirmOpen=",
  "handleLocalReferenceImageChange=", "referenceImageURL=", "setReferenceImageURL=", "handleAddNetworkReferenceImage=",
  "handleCancelCreateTaskConfirm=", "handleConfirmCreateTask="
];

let appJSX = appText.substring(appText.indexOf('<HomePage'));
let homePageTag = appJSX.substring(0, appJSX.indexOf('/>') + 2);
let newHomePageTag = homePageTag;
for (const prop of propsToRemove) {
    const regex = new RegExp(`\\s+${prop.replace('=', '')}=\\{[^}]+\\}`);
    newHomePageTag = newHomePageTag.replace(regex, '');
}
appText = appText.replace(homePageTag, newHomePageTag);
fs.writeFileSync(APP_TSX_PATH, appText);

// Inject into HomePage.tsx
const injectIdx = homeContent.indexOf('export function HomePage(props: HomePageProps) {') + 'export function HomePage(props: HomePageProps) {'.length;
const toInject = '\n  ' + extractedStates.join('\n  ') + '\n\n  ' + extractedFuncs.join('\n\n  ') + '\n';
let newHomeContent = homeContent.substring(0, injectIdx) + toInject + homeContent.substring(injectIdx);

// Modify HomePage Props Signature
for (const prop of propsToRemove) {
    const propName = prop.replace('=', '');
    const regex = new RegExp(`\\s+${propName}:\\s*[^;]+;`);
    newHomeContent = newHomeContent.replace(regex, '');
    const destructureRegex = new RegExp(`\\b${propName},\\s*`);
    newHomeContent = newHomeContent.replace(destructureRegex, '');
}
// Clean up any remaining trailing commas or empty lines in props destructure
newHomeContent = newHomeContent.replace(/,\s+}/g, ' }');

// Add dialogs to the end of HomePage return block
const homeReturnEnd = newHomeContent.lastIndexOf('</div>\n  );');
const dialogs = extractedFuncs.filter(f => f.startsWith('/* RENDERS */'))[0];
if (dialogs && homeReturnEnd !== -1) {
    newHomeContent = newHomeContent.substring(0, homeReturnEnd) + dialogs.replace('/* RENDERS */\n', '\n') + '\n' + newHomeContent.substring(homeReturnEnd);
}

// Remove the /* RENDERS */ from the middle
newHomeContent = newHomeContent.replace(/\/\* RENDERS \*\/[\s\S]*?\) : null}/g, '');

fs.writeFileSync(HOME_PAGE_PATH, newHomeContent);

console.log("Migration script executed successfully!");
