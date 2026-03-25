import { createPinia } from "pinia";
import { createApp } from "vue";

import App from "./App.vue";
import router from "./router";
import "./style.css";

function forceLightMode() {
  const removeDarkClass = () => {
    document.documentElement.classList.remove("dark");
    document.body.classList.remove("dark");
  };

  removeDarkClass();
  document.documentElement.style.colorScheme = "only light";
  document.body.style.colorScheme = "only light";

  const observer = new MutationObserver(removeDarkClass);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  observer.observe(document.body, { attributes: true, attributeFilter: ["class"] });
}

forceLightMode();

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.mount("#app");
