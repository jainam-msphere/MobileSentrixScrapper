async function checkBotProtection() {
  const url = "https://www.devicespecifications.com/en/model/2b2e671f";
  // const url = "https://www.phonemore.com/specs/samsung/galaxy-s26/";

  try {
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "User-Agent":
          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36",
      },
    });

    console.log("Status:", response.status);
    console.log("OK:", response.ok);

    console.log("\n=== HEADERS ===");
    response.headers.forEach((value, key) => {
      console.log(`${key}: ${value}`);
    });

    const html = await response.text();

    console.log("\n=== HTML PREVIEW ===");
    console.log(html);

    const botIndicators = [
      "verify you are human",
      "captcha",
      "checking your browser",
      "access denied",
      "enable javascript",
      "cloudflare",
      "datadome",
      "perimeterx",
      "attention required",
      "navigator.webdriver",
    ];

    const lowerHtml = html.toLowerCase();

    const detected = botIndicators.filter((indicator) =>
      lowerHtml.includes(indicator),
    );

    console.log("\n=== BOT CHECK ===");

    if (detected.length > 0) {
      console.log("Bot protection detected:");
      detected.forEach((d) => console.log("-", d));
    } else {
      console.log("No obvious bot protection detected.");
    }
  } catch (err) {
    console.error("Request failed:", err);
  }
}

checkBotProtection();
