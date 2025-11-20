// script.js: Interaction logic for navigation and cookie banner
document.addEventListener("DOMContentLoaded", () => {
  const navToggle = document.querySelector(".nav-toggle");
  const siteNav = document.querySelector(".site-nav");
  const navLinks = document.querySelectorAll(".site-nav a");
  const cookieBanner = document.querySelector(".cookie-banner");
  const cookieButton = document.getElementById("cookieAccept");

  if (navToggle && siteNav) {
    navToggle.addEventListener("click", () => {
      const expanded = navToggle.getAttribute("aria-expanded") === "true";
      navToggle.setAttribute("aria-expanded", String(!expanded));
      siteNav.classList.toggle("open");
    });

    navLinks.forEach((link) => {
      link.addEventListener("click", () => {
        siteNav.classList.remove("open");
        navToggle.setAttribute("aria-expanded", "false");
      });
    });
  }

  if (cookieBanner && cookieButton) {
    cookieButton.addEventListener("click", () => {
      cookieBanner.classList.add("hidden");
    });
  }
});

