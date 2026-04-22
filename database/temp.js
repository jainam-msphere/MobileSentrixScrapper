import { readFile, writeFile } from "fs";

let res = {};
readFile("./data.json", "utf8", (err, rawData) => {
  if (err) throw err;

  // 1. Parse the string into a usable object
  const data = JSON.parse(rawData);

  // 2. Iterate through "title" keys
  Object.entries(data).forEach(([mainKey, subTitles]) => {
    let arr = [];

    // 3. Iterate through "subtitle" keys and collect their items
    if (typeof subTitles === "object" && subTitles !== null) {
      Object.values(subTitles).forEach((items) => {
        if (Array.isArray(items)) {
          // Use spread operator to push all elements at once
          arr.push(...items);
        }
      });
    }

    res[mainKey] = arr;
  });

  console.log(res);
  writeFile("./phoneList.json", JSON.stringify(res), (err) => {
    if (err) throw err;
    console.log("File saved!");
  });
});
