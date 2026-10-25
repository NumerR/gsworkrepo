// Тема страницы до первой отрисовки: light/dark из localStorage, иначе из системной.
// Вынесено в отдельный файл, потому что CSP запрещает инлайновые скрипты.
(function () {
    var theme = localStorage.getItem("theme");
    if (!theme) {
        theme = matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    }
    document.documentElement.dataset.theme = theme;
})();
