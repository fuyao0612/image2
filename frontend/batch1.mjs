import fs from 'fs';
import path from 'path';

const APP_TSX = path.join(process.cwd(), 'src/App.tsx');
const HOME_TSX = path.join(process.cwd(), 'src/pages/HomePage.tsx');

let appContent = fs.readFileSync(APP_TSX, 'utf-8');
let homeContent = fs.readFileSync(HOME_TSX, 'utf-8');

const statesToMove = [
  "const [prompt, setPrompt] = useState('')",
  "const [size, setSize] = useState('1024x1024')",
  "const [style, setStyle] = useState('')",
  "const [count, setCount] = useState(1)",
  "const [quality, setQuality] = useState('auto')",
  "const [outputFormat, setOutputFormat] = useState('default')",
  "const [customWidth, setCustomWidth] = useState('1024')",
  "const [customHeight, setCustomHeight] = useState('1024')",
  "const [referenceImageURL, setReferenceImageURL] = useState('')",
  "const [referenceImagesDialogOpen, setReferenceImagesDialogOpen] = useState(false)",
  "const [showNetworkImageInput, setShowNetworkImageInput] = useState(false)",
  "const [promptDragActive, setPromptDragActive] = useState(false)"
];

for (const s of statesToMove) {
  appContent = appContent.replace(s, "");
}

// Add homeStateRef to App.tsx
appContent = appContent.replace("function App() {", "function App() {\n    const homeStateRef = useRef<any>({})");

// Replace all usages of these states in App.tsx with homeStateRef.current.XXX
const propsToReplace = [
  "prompt", "size", "customWidth", "customHeight", "style", "count", "quality", "outputFormat",
  "showNetworkImageInput", "referenceImageURL", "promptDragActive", "referenceImagesDialogOpen"
];
// We only want to replace them where they are USED in handlers. 
// But simple string replacement might be dangerous.
// Actually, they are only used in `estimatedGenerationPointCost`, `handleOpenCreateTaskConfirm`, `handleCreateTask`, `handleClearPromptOptions`, `RechargeConfirmDialog`, `ReferenceImagesDialog`, etc.

// Let's manually replace them in App.tsx to point to homeStateRef.current
appContent = appContent.replace(/!prompt\.trim\(\)/g, "!homeStateRef.current.prompt?.trim()");
appContent = appContent.replace(/const promptText = prompt\.trim\(\)/g, "const promptText = homeStateRef.current.prompt?.trim() || ''");
appContent = appContent.replace(/validateSelectedSize\(size, customWidth, customHeight\)/g, "validateSelectedSize(homeStateRef.current.size, homeStateRef.current.customWidth, homeStateRef.current.customHeight)");
appContent = appContent.replace(/resolveSelectedSize\(size, customWidth, customHeight\)/g, "resolveSelectedSize(homeStateRef.current.size, homeStateRef.current.customWidth, homeStateRef.current.customHeight)");
appContent = appContent.replace(/clampCount\(count\)/g, "clampCount(homeStateRef.current.count)");
appContent = appContent.replace(/style,/g, "style: homeStateRef.current.style,");
appContent = appContent.replace(/quality,/g, "quality: homeStateRef.current.quality,");
appContent = appContent.replace(/outputFormat,/g, "outputFormat: homeStateRef.current.outputFormat,");
appContent = appContent.replace(/referenceImageURL\.trim\(\)/g, "homeStateRef.current.referenceImageURL?.trim()");
appContent = appContent.replace(/setPromptDragActive/g, "homeStateRef.current.setPromptDragActive");
appContent = appContent.replace(/setPrompt\(/g, "homeStateRef.current.setPrompt(");
appContent = appContent.replace(/setReferenceImagesDialogOpen/g, "homeStateRef.current.setReferenceImagesDialogOpen");
appContent = appContent.replace(/setShowNetworkImageInput/g, "homeStateRef.current.setShowNetworkImageInput");
appContent = appContent.replace(/setReferenceImageURL/g, "homeStateRef.current.setReferenceImageURL");

// We must also update `count` and `estimatedGenerationPointCost` inside `RechargeConfirmDialog`.
appContent = appContent.replace(/clampCount\(Number\(count\)/g, "clampCount(Number(homeStateRef.current.count)");
appContent = appContent.replace(/clampCount\(Number\(count\)/g, "clampCount(Number(homeStateRef.current.count)"); // just in case

// Add homeStateRef to HomePage
appContent = appContent.replace(/<HomePage/g, '<HomePage homeStateRef={homeStateRef}');

// Remove the props from HomePage in App.tsx
const propsToRemoveFromJSX = [
  "prompt=", "setPrompt=", "size=", "setSize=", "customWidth=", "setCustomWidth=", "customHeight=", "setCustomHeight=",
  "style=", "setStyle=", "count=", "setCount=", "quality=", "setQuality=", "outputFormat=", "setOutputFormat=",
  "showNetworkImageInput=", "setShowNetworkImageInput=", "referenceImageURL=", "setReferenceImageURL=",
  "promptDragActive=", "referenceImagesDialogOpen=", "setReferenceImagesDialogOpen="
];

let appJSX = appContent.substring(appContent.indexOf('<HomePage'));
let homePageTag = appJSX.substring(0, appJSX.indexOf('/>') + 2);
let newHomePageTag = homePageTag;
for (const prop of propsToRemoveFromJSX) {
    const regex = new RegExp(`\\s+${prop}\\{[^}]+\\}`);
    newHomePageTag = newHomePageTag.replace(regex, '');
}
appContent = appContent.replace(homePageTag, newHomePageTag);
fs.writeFileSync(APP_TSX, appContent);

// Inject into HomePage.tsx
const injectIdx = homeContent.indexOf('export function HomePage(props: HomePageProps) {') + 'export function HomePage(props: HomePageProps) {'.length;
const toInject = '\n  ' + statesToMove.join('\n  ') + '\n' +
  `
  useEffect(() => {
    if (props.homeStateRef) {
      props.homeStateRef.current = {
        prompt, setPrompt, size, setSize, customWidth, setCustomWidth, customHeight, setCustomHeight,
        style, setStyle, count, setCount, quality, setQuality, outputFormat, setOutputFormat,
        showNetworkImageInput, setShowNetworkImageInput, referenceImageURL, setReferenceImageURL,
        promptDragActive, setPromptDragActive, referenceImagesDialogOpen, setReferenceImagesDialogOpen
      }
    }
  })
`;
let newHomeContent = homeContent.substring(0, injectIdx) + toInject + homeContent.substring(injectIdx);

// Modify HomePage Props Signature
for (const prop of propsToRemoveFromJSX) {
    const propName = prop.replace('=', '');
    const regex = new RegExp(`\\s+${propName}:\\s*[^;]+;`);
    newHomeContent = newHomeContent.replace(regex, '');
    const destructureRegex = new RegExp(`\\b${propName},\\s*`);
    newHomeContent = newHomeContent.replace(destructureRegex, '');
}
newHomeContent = newHomeContent.replace(/,\s+}/g, ' }');
newHomeContent = newHomeContent.replace('export interface HomePageProps {', 'export interface HomePageProps {\n  homeStateRef?: any;');

fs.writeFileSync(HOME_TSX, newHomeContent);

console.log("Batch 1 migration complete!");
