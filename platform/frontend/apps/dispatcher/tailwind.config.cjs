const config = require("@cbweb3/config/tailwind-preset");

/** @type {import('tailwindcss').Config} */
module.exports = {
  presets: [config],
  content: [
    "./index.html",
    "./src/**/*.{ts,tsx}",
    "../../packages/ui/src/**/*.{ts,tsx}",
  ],
  darkMode: ["class"],
};
