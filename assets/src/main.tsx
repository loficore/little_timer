import { render } from "preact";
import { App } from "./App.tsx";
import "./styles/globals.css";
import { setLanguage } from "./utils/i18n";

const root = document.getElementById("root") as HTMLElement;

// 默认加载中文语言包，避免初次渲染时出现空白文本
await setLanguage("ZH");
render(<App />, root);
