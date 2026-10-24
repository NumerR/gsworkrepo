// Подсвечиваем пункт навигации текущей страницы
document.querySelectorAll(".nav-link").forEach((link) => {
    const href = link.getAttribute("href");
    const path = location.pathname;

    if (href === "/" ? path === "/" : path.startsWith(href)) {
        link.setAttribute("aria-current", "page");
    }
});

// Ripple: волна Material из точки нажатия на кнопках и пунктах навигации
document.addEventListener("pointerdown", (e) => {
    const host = e.target.closest(".btn, .icon-btn, .nav-link");
    if (!host) return;

    const rect = host.getBoundingClientRect();
    const size = Math.max(rect.width, rect.height);

    const ripple = document.createElement("span");
    ripple.className = "ripple";
    ripple.style.width = ripple.style.height = size + "px";
    ripple.style.left = e.clientX - rect.left - size / 2 + "px";
    ripple.style.top = e.clientY - rect.top - size / 2 + "px";

    host.appendChild(ripple);
    ripple.addEventListener("animationend", () => ripple.remove(), { once: true });
    // Страховка: в скрытой вкладке анимации не тикают и animationend может не прийти
    setTimeout(() => ripple.remove(), 700);
});

// Переключение светлой и тёмной темы
(function () {
    const root = document.documentElement;
    const toggle = document.getElementById("theme-toggle");
    if (!toggle) return;

    const icon = toggle.querySelector(".icon");

    function apply(theme, animate) {
        root.dataset.theme = theme;
        icon.textContent = theme === "dark" ? "light_mode" : "dark_mode";
        toggle.setAttribute(
            "aria-label",
            theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему",
        );

        if (animate) {
            icon.classList.remove("icon-swap");
            void icon.offsetWidth;
            icon.classList.add("icon-swap");
        }
    }

    apply(root.dataset.theme === "dark" ? "dark" : "light", false);

    toggle.addEventListener("click", () => {
        const theme = root.dataset.theme === "dark" ? "light" : "dark";
        // Запоминаем только выбор пользователя, иначе системная тема перестанет учитываться
        localStorage.setItem("theme", theme);
        apply(theme, true);
    });

    // Пока пользователь не выбрал тему сам, следуем за системной
    matchMedia("(prefers-color-scheme: dark)").addEventListener("change", (e) => {
        if (!localStorage.getItem("theme")) apply(e.matches ? "dark" : "light", true);
    });
})();

// ============================================================
// Motion: добавление / изменение / удаление трат
// ============================================================

const reducedMotion = matchMedia("(prefers-reduced-motion: reduce)").matches;

// Рука-курсор: заменяет системный курсор на сайте и плавно «болтается»
// за указателем на пружине, с наклоном по ходу движения
(function () {
    if (reducedMotion || !matchMedia("(hover: hover) and (pointer: fine)").matches) return;

    // картинка 114x123, кончик поднятого пальца (хотспот) — в (31, 0)
    const HAND = { w: 114, hotX: 31, hotY: 0 };
    const shown = 32; // px по ширине
    const k = shown / HAND.w;

    const hand = document.createElement("img");
    hand.className = "cursor-hand";
    hand.src = "/static/images/cursor.png";
    hand.alt = "";
    hand.style.transformOrigin = `${(HAND.hotX * k).toFixed(1)}px ${(HAND.hotY * k).toFixed(1)}px`;
    document.body.appendChild(hand);

    // системный курсор прячем только когда рука точно загрузилась
    hand.addEventListener("load", () => document.documentElement.classList.add("custom-cursor"));

    let tx = innerWidth / 2;
    let ty = innerHeight / 2;
    let x = tx;
    let y = ty;
    let vx = 0;
    let vy = 0;
    let angle = 0;
    let scale = 1;
    let hover = 0;
    let hoverTarget = 0;
    let down = false;

    addEventListener("pointermove", (e) => {
        tx = e.clientX;
        ty = e.clientY;
        hand.classList.add("is-on");
        hoverTarget = e.target.closest("a, button, input, label, select") ? 1 : 0;
    }, { passive: true });
    addEventListener("pointerdown", () => { down = true; });
    addEventListener("pointerup", () => { down = false; });
    document.addEventListener("pointerleave", () => hand.classList.remove("is-on"));
    addEventListener("blur", () => hand.classList.remove("is-on"));

    const STIFF = 0.16; // жёсткость пружины: тянемся к указателю
    const DAMP = 0.68; // и «болтаемся» с перелётом
    const BRK = 3; // сила доворота при торможении
    let prevSpeed = 0;
    (function loop() {
        vx = (vx + (tx - x) * STIFF) * DAMP;
        vy = (vy + (ty - y) * STIFF) * DAMP;
        x += vx;
        y += vy;

        // маятник: наклон пропорционален горизонтальной скорости,
        // а при потере скорости рука доворачивается по ходу движения
        const speed = Math.abs(vx);
        const brake = Math.max(0, prevSpeed - speed) * Math.sign(vx);
        prevSpeed = speed;

        const sway = Math.max(-24, Math.min(24, -vx * 1.6));
        const nod = Math.max(-8, Math.min(8, brake * BRK));
        angle += (sway + nod - angle) * 0.15;

        hover += (hoverTarget - hover) * 0.12;
        const target = (down ? 0.82 : 1) * (1 + hover * 0.12);
        scale += (target - scale) * 0.2;

        hand.style.transform =
            `translate(${x - HAND.hotX * k}px, ${y - HAND.hotY * k}px) ` +
            `rotate(${angle.toFixed(2)}deg) scale(${scale.toFixed(3)})`;
        requestAnimationFrame(loop);
    })();
})();

// Разлетающиеся квадратные частицы от центра элемента (как на референсе)
function burstFrom(el) {
    const rect = el.getBoundingClientRect();
    const cx = rect.left + rect.width / 2;
    const cy = rect.top + rect.height / 2;

    let layer = document.querySelector(".burst-layer");
    if (!layer) {
        layer = document.createElement("div");
        layer.className = "burst-layer";
        document.body.appendChild(layer);
    }

    const dark = document.documentElement.dataset.theme === "dark";
    const colors = dark
        ? ["#d0bcff", "#82b1ff", "#b39ddb", "#7c4dff", "#e6c7ff", "#448aff"]
        : ["#6750a4", "#7c4dff", "#3f51b5", "#b39ddb", "#448aff", "#d0bcff"];
    for (let i = 0; i < 18; i++) {
        const p = document.createElement("i");
        p.className = "burst-particle";
        p.style.left = cx + "px";
        p.style.top = cy + "px";
        p.style.background = colors[i % colors.length];
        const size = 6 + Math.random() * 6;
        p.style.width = p.style.height = size + "px";
        layer.appendChild(p);

        const angle = Math.random() * Math.PI * 2;
        const dist = 60 + Math.random() * 110;
        const dx = Math.cos(angle) * dist;
        const dy = Math.sin(angle) * dist * 0.8 - 30;
        const rot = (Math.random() - 0.5) * 720;

        p.animate(
            [
                { transform: "translate(-50%, -50%) scale(1)", opacity: 1 },
                {
                    transform: `translate(calc(-50% + ${dx * 0.6}px), calc(-50% + ${dy * 0.6}px)) rotate(${rot * 0.5}deg)`,
                    opacity: 1,
                    offset: 0.4,
                },
                {
                    transform: `translate(calc(-50% + ${dx}px), calc(-50% + ${dy + 70}px)) rotate(${rot}deg) scale(.6)`,
                    opacity: 0,
                },
            ],
            { duration: 650 + Math.random() * 350, easing: "cubic-bezier(.2, .6, .35, 1)", fill: "forwards" },
        );
        setTimeout(() => p.remove(), 1100);
    }
    setTimeout(() => {
        if (layer && !layer.childElementCount) layer.remove();
    }, 1300);
}

// Растворение текста кнопки по буквам, возвращает длительность в мс
function dissolveLabel(label) {
    const text = label.textContent;
    label.textContent = "";
    const spans = [...text].map((ch) => {
        const s = document.createElement("span");
        s.textContent = ch;
        s.style.display = "inline-block";
        s.style.whiteSpace = "pre";
        label.appendChild(s);
        return s;
    });
    spans.forEach((s, i) => {
        s.animate(
            [
                { opacity: 1, transform: "none" },
                { opacity: 0, transform: "translateY(-10px) rotate(8deg) scale(.7)" },
            ],
            { duration: 260, delay: i * 22, easing: "ease-in", fill: "forwards" },
        );
    });
    return spans.length * 22 + 260;
}

// Кнопка «Добавить/Сохранить»: текст растворяется, взрыв конфетти, галочка «Готово»
document.querySelectorAll("form.form").forEach((form) => {
    form.addEventListener("submit", (e) => {
        if (form.dataset.busy) {
            e.preventDefault(); // пока идёт анимация, повторяющая отправка блокируется
            return;
        }
        if (reducedMotion) return;
        e.preventDefault();
        form.dataset.busy = "1";

        const btn = form.querySelector("button[type=submit]");
        const label = btn.querySelector(".btn-label");
        const icon = btn.querySelector(".icon");
        btn.style.minWidth = btn.offsetWidth + "px";
        btn.setAttribute("aria-busy", "true");

        const dissolveMs = label ? dissolveLabel(label) : 0;

        setTimeout(() => {
            if (icon) icon.textContent = "check";
            if (label) label.textContent = "Готово";
            btn.classList.add("is-success");
            burstFrom(btn);
        }, dissolveMs);

        // form.submit() идёт мимо обработчика и не запускает анимацию заново
        setTimeout(() => form.submit(), dissolveMs + 650);
    });
});

// Hold to Delete: удержание кнопки закрашивает пилюлю, по заполнению — удаление
(function () {
    const btn = document.querySelector("[data-hold-form]");
    if (!btn) return;

    const label = btn.querySelector(".hold-label");
    if (reducedMotion) return; // остаётся обычная кнопка-submit с подписью «Удалить»

    const form = btn.closest("form");
    const HOLD_MS = 1100;
    let holding = false;
    let done = false;
    let start = 0;
    let raf = 0;
    let iv = 0;

    btn.type = "button";
    btn.classList.add("hold-armed");
    btn.setAttribute("aria-label", "Удерживайте кнопку, чтобы удалить трату");
    label.textContent = "Удерживайте для удаления";
    label.dataset.text = label.textContent;

    function tick() {
        if (!holding || done) return;
        const p = Math.min(1, (performance.now() - start) / HOLD_MS);
        btn.style.setProperty("--hold", p);
        if (p < 1) return;

        holding = false;
        done = true;
        clearInterval(iv);
        btn.classList.add("hold-done");
        setTimeout(() => form.requestSubmit(), 180);
    }

    function begin(e) {
        if (done || holding) return;
        holding = true;
        btn.style.transition = "none";
        start = performance.now();
        try {
            // указатель «прилипает» к кнопке: отпускание вне неё тоже отменяет удержание
            btn.setPointerCapture(e.pointerId);
        } catch {
            /* синтетические события без указателя */
        }
        (function loop() {
            if (!holding || done) return;
            tick();
            raf = requestAnimationFrame(loop);
        })();
        // Страховка: в фоновой вкладке rAF засыпает, а прогресс считается по часам
        iv = setInterval(tick, 100);
    }

    function end() {
        if (!holding || done) return;
        holding = false;
        cancelAnimationFrame(raf);
        clearInterval(iv);
        btn.style.transition = "";
        btn.style.setProperty("--hold", 0);
    }

    btn.addEventListener("pointerdown", begin);
    btn.addEventListener("pointerup", end);
    btn.addEventListener("pointercancel", end);
    btn.addEventListener("click", (e) => e.preventDefault());
    btn.addEventListener("contextmenu", end);
    // клавиатурное удержание не должно «дожигать» после потери фокуса страницы
    addEventListener("blur", end);
    document.addEventListener("visibilitychange", () => {
        if (document.hidden) end();
    });
    btn.addEventListener("keydown", (e) => {
        if (e.code === "Space" || e.code === "Enter") {
            e.preventDefault();
            if (!e.repeat) begin(e);
        }
    });
    btn.addEventListener("keyup", (e) => {
        if (e.code === "Space" || e.code === "Enter") {
            e.preventDefault();
            end();
        }
    });
})();

// Подсветка строки после добавления/изменения (приходит через ?flash=...&id=...)
(function () {
    const params = new URLSearchParams(location.search);
    const flash = params.get("flash");
    const id = params.get("id");
    if (flash !== "add" && flash !== "edit") return;

    if (!reducedMotion && /^\d+$/.test(id || "")) {
        const row = document.querySelector(`tr[data-id="${id}"]`);
        if (row) {
            row.classList.add(flash === "add" ? "row-enter" : "row-update");
            row.scrollIntoView({ block: "center", behavior: "smooth" });
        }
    }
    history.replaceState(null, "", location.pathname);
})();

// Кнопка удаления в списке: строка вылетает, соседние плавно сдвигаются (FLIP),
// и только потом переход на страницу подтверждения
(function () {
    document.addEventListener("click", (e) => {
        const link = e.target.closest("tr[data-id] .icon-btn-danger");
        if (!link || reducedMotion || link.dataset.gone) return;
        e.preventDefault();
        link.dataset.gone = "1";

        const row = link.closest("tr");
        const others = [...document.querySelectorAll("tr[data-id]")].filter((r) => r !== row);
        const before = others.map((r) => r.getBoundingClientRect());

        let navigated = false;
        function go() {
            if (navigated) return;
            navigated = true;
            location.href = link.href;
        }

        function settle() {
            if (row.isConnected) row.remove();
            let maxShift = 0;
            others.forEach((r, i) => {
                const dy = before[i].top - r.getBoundingClientRect().top;
                maxShift = Math.max(maxShift, Math.abs(dy));
                if (dy) {
                    r.animate([{ transform: `translateY(${dy}px)` }, { transform: "none" }], {
                        duration: 320,
                        easing: "cubic-bezier(.2, 0, 0, 1)",
                    });
                }
            });
            // без сдвига (удалялась последняя строка) можно не ждать
            setTimeout(go, maxShift ? 340 : 0);
        }

        row.classList.add("row-exit");
        row.addEventListener("animationend", (ev) => {
            if (ev.animationName === "row-exit") settle();
        }, { once: true });
        // Страховка: в скрытой вкладке animationend может не прийти
        setTimeout(settle, 450);
    });
})();
