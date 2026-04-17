import { render } from "preact";
import { App } from "./App.tsx";
import "./styles/globals.css";
import { setLanguage } from "./utils/i18n";
import { STORAGE_KEYS } from "./utils/constants";

const root = document.getElementById("root") as HTMLElement;

const applyLightStyleClass = () => {
	const html = document.documentElement;
	const search = new URLSearchParams(window.location.search);
	const styleFromQuery = search.get("lightStyle");
	const styleFromStorage = localStorage.getItem(STORAGE_KEYS.LIGHT_STYLE);
	const style = (styleFromQuery || styleFromStorage || "paper").toLowerCase();

	html.classList.remove("light-style-mist");
	if (style === "mist") {
		html.classList.add("light-style-mist");
	}
};

applyLightStyleClass();

// 默认加载中文语言包，避免初次渲染时出现空白文本
await setLanguage("ZH");
render(<App />, root);
