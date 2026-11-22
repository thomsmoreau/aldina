(() => {
	// ------------------------------------------------------------------
	// Configuration
	// ------------------------------------------------------------------
	// Set this to a real form endpoint (Formspree, Basin, etc.) to make
	// the signup actually send the address. On a static OVH host this is
	// the only way to collect emails without a backend.
	const CONFIG = {
		formEndpoint: "", // e.g. 'https://formspree.io/f/YOUR_FORM_ID'
	};

	// localStorage throws in private mode and when cookies are blocked, so
	// every access goes through these.
	const safeRead = (key) => {
		try {
			return localStorage.getItem(key);
		} catch (e) {
			return null;
		}
	};

	const safeStore = (key, value) => {
		try {
			localStorage.setItem(key, value);
		} catch (e) {
			/* ignore */
		}
	};

	// ------------------------------------------------------------------
	// i18n — plain text only, no HTML, so we can use textContent safely.
	// ------------------------------------------------------------------
	// Brand and domain terms that stay identical in every language. Anything a
	// translator would touch belongs in the per-language block, even if two
	// languages happen to agree today.
	const COMMON = {
		foot1_prefix: "Aldina — ",
		lane2: "Aldina",
		foot_heritage:
			"ProfileAsCode · compatible OrcaSlicer, PrusaSlicer, BambuStudio",
		p_in1: "Filament",
		p_in2: "Process",
		p_in3: "Machine",
	};

	const I18N = {
		fr: {
			...COMMON,
			title: "Aldina — Définissez votre profil une fois. Imprimez partout.",
			m_soon: "bientôt",
			m_notify: "Être informé",
			hero_eyebrow: "ProfileAsCode · impression 3D",
			hero_h1_1: "Définissez votre profil une fois.",
			hero_h1_2: "Imprimez partout.",
			hero_lede:
				"Définissez une seule fois vos profils filament, process et machine. Aldina les exporte pour chaque slicer et chaque imprimante, sans modification.",
			hero_btn1: "Être prévenu au lancement",
			hero_btn2: "Voir le concept",
			press_h2_1: "Écrit une fois.",
			press_h2_2: "Converti pour tous.",
			press_lede:
				"Le même profil, généré au format de chaque slicer — sans rien réécrire.",
			lane1: "Profils source",
			lane3: "Tous les slicers",
			press_caption:
				"Un profil source en entrée ; en sortie, un format fidèle par slicer, utilisable sur n'importe quelle imprimante.",
			universal_eyebrow: "Universel par conception",
			universal_h2_1: "Un profil source.",
			universal_h2_2: "Toutes les imprimantes.",
			universal_lede:
				"Un profil créé pour une imprimante et un slicer fonctionne tel quel sur une autre imprimante, avec un autre slicer.",
			universal_b1: "Profil filament",
			universal_p1: "Températures, débit, refroidissement…",
			universal_b2: "Profil process",
			universal_p2: "Vitesse, remplissage, supports…",
			universal_b3: "Profil machine",
			universal_p3: "Volume, cinématique, buses…",
			universal_summary:
				"Vos profils restent cohérents, quel que soit le slicer ou l'imprimante cible — sans modification ni réécriture.",
			access_eyebrow: "Bientôt",
			access_h2_1: "Aldina",
			access_h2_2: "bientôt disponible.",
			access_lede: "Inscrivez votre e-mail pour être informé du lancement.",
			access_btn: "M'inscrire →",
			access_micro: "Aucun spam. Un seul e-mail, le jour du lancement.",
			done_msg: "Merci — vous serez informé du lancement.",
			foot1_status: "bientôt",
			done_ok: "✓ confirmé",
			fallback_msg:
				"Aucun backend configuré — votre email est mémorisé dans ce navigateur.",
			error_invalid: "Veuillez saisir une adresse email valide.",
			error_send: "L'envoi a échoué. Veuillez réessayer.",
			ph: "vous@atelier.fr",
		},
		en: {
			...COMMON,
			title: "Aldina — Perfect it once. Print anywhere.",
			m_soon: "soon",
			m_notify: "Get notified",
			hero_eyebrow: "ProfileAsCode · 3D printing",
			hero_h1_1: "Perfect it once.",
			hero_h1_2: "Print anywhere.",
			hero_lede:
				"Define your filament, process, and machine profiles once. Aldina exports them for every slicer and every printer — no rework.",
			hero_btn1: "Get notified at launch",
			hero_btn2: "See the concept",
			press_h2_1: "Written once.",
			press_h2_2: "Translated for all.",
			press_lede:
				"The same profile, rendered in every slicer's format — without rewriting a thing.",
			lane1: "Source profiles",
			lane3: "Every slicer",
			press_caption:
				"One source profile in; a faithful format out for every slicer, ready for any printer.",
			universal_eyebrow: "Universal by design",
			universal_h2_1: "One source profile.",
			universal_h2_2: "Every printer.",
			universal_lede:
				"A profile built for one printer and slicer works as-is on another printer with another slicer.",
			universal_b1: "Filament profile",
			universal_p1: "Temperatures, flow, cooling…",
			universal_b2: "Process profile",
			universal_p2: "Speed, infill, supports…",
			universal_b3: "Machine profile",
			universal_p3: "Volume, kinematics, nozzles…",
			universal_summary:
				"Your profiles stay consistent across any slicer and target printer — no rework, no rewrite.",
			access_eyebrow: "Soon",
			access_h2_1: "Aldina is",
			access_h2_2: "launching soon.",
			access_lede: "Enter your email to be notified when we launch.",
			access_btn: "Join the waitlist →",
			access_micro: "No spam. One email on launch day.",
			done_msg: "Thanks — we'll notify you at launch.",
			foot1_status: "soon",
			done_ok: "✓ confirmed",
			fallback_msg:
				"No backend configured — your email is saved in this browser.",
			error_invalid: "Please enter a valid email address.",
			error_send: "Submission failed. Please try again.",
			ph: "you@studio.com",
		},
	};

	let currentLang =
		safeRead("aldina_lang") ||
		((navigator.language || "fr").toLowerCase().startsWith("en") ? "en" : "fr");

	function applyLang(lang) {
		if (!I18N[lang]) lang = "fr";
		currentLang = lang;
		const d = I18N[lang];

		document.querySelectorAll("[data-i18n]").forEach((el) => {
			const k = el.getAttribute("data-i18n");
			if (d[k] != null) el.textContent = d[k];
		});

		document.querySelectorAll("[data-i18n-ph]").forEach((el) => {
			const k = el.getAttribute("data-i18n-ph");
			if (d[k] != null) el.setAttribute("placeholder", d[k]);
		});

		document.documentElement.setAttribute("lang", lang);
		if (d.title) document.title = d.title;

		document.querySelectorAll(".lang button").forEach((b) => {
			b.setAttribute(
				"aria-pressed",
				b.getAttribute("data-lang") === lang ? "true" : "false",
			);
		});

		safeStore("aldina_lang", lang);
	}

	document.querySelectorAll(".lang button").forEach((b) => {
		b.addEventListener("click", () => {
			applyLang(b.getAttribute("data-lang"));
		});
	});

	applyLang(currentLang);

	// ------------------------------------------------------------------
	// Early access form
	// ------------------------------------------------------------------
	const form = document.getElementById("signup");
	const done = document.getElementById("done");
	const doneMail = document.getElementById("done-mail");
	const doneExtra = document.getElementById("done-extra");
	const emailEl = document.getElementById("email");
	const errorEl = document.getElementById("signup-error");

	function showError(msg) {
		if (!errorEl) return;
		errorEl.textContent = msg;
		errorEl.classList.add("show");
		emailEl.setAttribute("aria-invalid", "true");
	}

	function clearError() {
		if (!errorEl) return;
		errorEl.textContent = "";
		errorEl.classList.remove("show");
		emailEl.removeAttribute("aria-invalid");
	}

	function showSuccess(email, extra) {
		doneMail.textContent = email;
		doneExtra.textContent = extra || "";
		form.style.display = "none";
		done.classList.add("show");
		safeStore("aldina_email", email);
	}

	function isValidEmail(v) {
		return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v);
	}

	// If the user already signed up in this browser, show the success state.
	const savedEmail = safeRead("aldina_email");
	if (savedEmail) {
		showSuccess(savedEmail, I18N[currentLang].fallback_msg);
	}

	if (form) {
		form.addEventListener("submit", (e) => {
			e.preventDefault();
			clearError();

			const v = (emailEl.value || "").trim();
			if (!isValidEmail(v)) {
				showError(I18N[currentLang].error_invalid);
				emailEl.focus();
				return;
			}

			// No endpoint configured: be honest and store locally.
			if (!CONFIG.formEndpoint) {
				showSuccess(v, I18N[currentLang].fallback_msg);
				return;
			}

			// Real endpoint configured: POST the form.
			fetch(CONFIG.formEndpoint, {
				method: "POST",
				body: new FormData(form),
				headers: { Accept: "application/json" },
			})
				.then((res) => {
					if (res.ok) {
						showSuccess(v);
					} else {
						throw new Error("bad response");
					}
				})
				.catch(() => {
					showError(I18N[currentLang].error_send);
				});
		});
	}

	// ------------------------------------------------------------------
	// Scroll reveal
	// ------------------------------------------------------------------
	const rvEls = [].slice.call(document.querySelectorAll(".rv"));
	const reduced =
		window.matchMedia &&
		window.matchMedia("(prefers-reduced-motion: reduce)").matches;

	if (reduced || !rvEls.length) {
		rvEls.forEach((el) => {
			el.classList.add("in");
		});
	} else {
		const reveal = () => {
			const vh = window.innerHeight || document.documentElement.clientHeight;
			for (let i = rvEls.length - 1; i >= 0; i--) {
				const el = rvEls[i];
				const r = el.getBoundingClientRect();
				if (r.top < vh * 0.92 && r.bottom > 0) {
					el.classList.add("in");
					rvEls.splice(i, 1);
				}
			}
		};
		reveal();
		window.addEventListener("scroll", reveal, { passive: true });
		window.addEventListener("resize", reveal);
		// safety: never leave content hidden
		setTimeout(() => {
			document.querySelectorAll(".rv").forEach((el) => {
				el.classList.add("in");
			});
		}, 4000);
	}
})();
