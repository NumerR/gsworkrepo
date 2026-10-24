// Подсвечиваем пункт навигации текущей страницы
document.querySelectorAll(".nav-link").forEach((link) => {
    const href = link.getAttribute("href");
    const path = location.pathname;

    if (href === "/" ? path === "/" : path.startsWith(href)) {
        link.setAttribute("aria-current", "page");
    }
});
