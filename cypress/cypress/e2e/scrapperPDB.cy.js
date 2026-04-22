/// <reference types="cypress" />

describe("admin components testing", () => {
  it("scrapping webpage for mobiles", () => {
    const MOBILE_COMPANY = "Apple"; // provide name of company here which are available in gsmarena
    // const PAGE_NUMBER = 4; //provide which pagination you would wish to extract data REMEMBER: PROVIDE THE PAGE NUMBER - 1. E.G. you want to extract data of all phones on 5th page then provide number 4

    const MOBILE_LINK =
      "https://phonedb.net/index.php?m=device&id=25160&c=apple_iphone_17_5g_uw_a3258_dual_sim_td-lte_us_512gb__apple_iphone_18,3";
    // "https://phonedb.net/index.php?m=device&id=25593&c=huawei_enjoy_70s_dual_sim_td-lte_cn_128gb_gfy-al00__changxiang_70_s__huawei_goofy&d=detailed_specs"
    const MOBILE_NAME = "device name here";
    const LINKS_ARRAY = ["", "", ""];
    cy.visit(MOBILE_LINK + "&d=detailed_specs");

    cy.get("div.container div.canvas table")
      .first()
      .should("be.visible")
      .wait(2000)
      .then(($table) => {
        cy.request("POST", "http://localhost:8080/extractPDB", {
          html: $table[0].outerHTML,
          phone: MOBILE_NAME,
          company: MOBILE_COMPANY,
        }).then((resp) => {
          expect(resp.status).to.eq(200);
        });
      });
  });
});

// cy.wrap(LINKS_ARRAY).each((v,i)=>{
//     console.log("link - ",v, " at index - ",i)
//     cy.visit(v + "&d=detailed_specs");
//     cy.get("div.container div.canvas table")
//       .first()
//       .should("be.visible")
//       .wait(2000)
//       .then(($table) => {
//         cy.wrap($table)
//           .contains("td", "Dimensions", { timeout: 10000 })
//           .should("exist")
//           .then(() => {
//             cy.request("POST", "http://localhost:8080/extractPDB", {
//               html: $table[0].outerHTML,
//               phone: MOBILE_NAME,
//               company: MOBILE_COMPANY,
//             }).then((resp) => {
//               expect(resp.status).to.eq(200);
//             });
//           });
//       });
// })
