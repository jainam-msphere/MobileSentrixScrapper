/// <reference types="cypress" />

describe("admin components testing", () => {
  it("scrapping webpage for mobiles", () => {
    const MOBILE_COMPANY = "Apple"; // provide name of company here which are available in gsmarena
    const PAGE_NUMBER = 4; //provide which pagination you would wish to extract data REMEMBER: PROVIDE THE PAGE NUMBER - 1. E.G. you want to extract data of all phones on 5th page then provide number 4

    cy.visit("https://www.gsmarena.com/");

    cy.get(".brandmenu-v2").should("be.visible");

    cy.get(".brandmenu-v2")
      .find("ul")
      .find("li")
      .contains("a", MOBILE_COMPANY)
      .should("be.visible")
      .click();
    cy.url().should("include", `${MOBILE_COMPANY.toLowerCase()}`);

    Cypress._.times(PAGE_NUMBER, () => {
      cy.get(".prevnextbutton").last().click();
    });

    cy.window().then((win) => {
      cy.url().then((currentUrl) => {
        win.sessionStorage.setItem("url", currentUrl);
      });
    });

    cy.get("div.makers ul li")
      .should("have.length.at.least", 1)
      .then(($li) => {
        const count = $li.length;
        cy.log(`Found ${count} phones in the makers list`);
        cy.window().then((win) => {
          win.sessionStorage.setItem("totalPhones", count);
        });
      });

    cy.window().then((win) => {
      Cypress._.times(
        Number(win.sessionStorage.getItem("totalPhones")),
        (i) => {
          cy.get("div.makers ul li").eq(i).find("a").click();
          cy.get("#specs-list").then(($el) => {
            cy.get("h1.specs-phone-name-title").then(($heading) => {
              cy.request("POST", "http://localhost:8080/extractGSM", {
                html: $el[0].outerHTML,
                phone: $heading[0].innerText,
                company: MOBILE_COMPANY,
              }).then((resp) => {
                expect(resp.status).to.eq(200);
              });
            });
          });

          cy.wait(5000);
          cy.visit(win.sessionStorage.getItem("url"));
        },
      );
    });
  });
});
