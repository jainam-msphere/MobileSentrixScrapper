/* eslint-disable react-hooks/set-state-in-effect */
import React, { useState, useEffect } from "react";

export default function DeviceForm() {
  const [manufacturers, setManufacturers] = useState([]);
  const [devices, setDevices] = useState([]);
  const [selectedManufacturer, setSelectedManufacturer] = useState("");
  const [selectedDevice, setSelectedDevice] = useState("");
  const [isFetchingDevices, setIsFetchingDevice] = useState(false);
  const [useLink, setUseLink] = useState(false);
  const [linkSource, setLinkSource] = useState("phonedb");
  const [linkUrl, setLinkUrl] = useState("");
  const [jsonOutput, setJsonOutput] = useState(null);
  const [isLoading, setIsLoading] = useState(false);

  useEffect(() => {
    fetch("http://localhost:8080/manufacturers")
      .then((res) => res.json())
      .then((data) => {
        let temp = [];
        data.results.forEach((e) => {
          temp.push(e.manufacturer);
        });
        setManufacturers(temp);
      })
      .catch((err) => console.error("Failed to fetch manufacturers", err));
  }, []);

  useEffect(() => {
    setSelectedManufacturer("");
    setSelectedDevice("");
  }, [useLink]);

  useEffect(() => {
    if (!selectedManufacturer) {
      setDevices([]);
      setSelectedDevice("");
      return;
    }

    if (!useLink) {
      setIsFetchingDevice(true);
      const query = new URLSearchParams({
        page_token: "n/a",
      }).toString();
      fetch(
        `http://localhost:8080/manufacturers/${encodeURIComponent(selectedManufacturer)}/devices?${query.toString()}`,
      )
        .then((res) => res.json())
        .then((data) => {
          setIsFetchingDevice(false);
          setDevices(data.results[0].devices);
        })
        .catch((err) => console.error("Failed to fetch devices", err));
    }
  }, [selectedManufacturer, useLink]);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsLoading(true);

    const params = {
      source_name: linkSource,
      link: linkUrl,
    };

    if (useLink) {
      params.source = linkSource;
      params.url = linkUrl;
    }
    if (useLink && (params.link == "" || params.source_name == "")) {
      console.log("link or source name not provided");
      return;
    }
    const queryString = new URLSearchParams(params).toString();

    try {
      let response;
      if (useLink) {
        response = await fetch(
          `http://localhost:8080/brands/${encodeURIComponent(selectedManufacturer)}/devices/${encodeURIComponent(selectedDevice)}/sources/link?${queryString}`,
        );
      } else {
        response = await fetch(
          `http://localhost:8080/brands/${encodeURIComponent(selectedManufacturer)}/devices/${encodeURIComponent(selectedDevice)}/sources/data`,
        );
      }
      const data = await response.json();
      setJsonOutput(data);
    } catch (error) {
      console.error("Submission failed", error);
      setJsonOutput({ error: "Failed to fetch data" });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div
      style={{ maxWidth: "600px", margin: "0 auto", fontFamily: "sans-serif" }}
    >
      <h2>Device Lookup Form</h2>

      <form
        onSubmit={handleSubmit}
        style={{ display: "flex", flexDirection: "column", gap: "15px" }}
      >
        {/* --- REQUIRED DROPDOWNS --- */}
        <div>
          <label style={{ display: "block", marginBottom: "5px" }}>
            Manufacturer *
          </label>
          <select
            required
            value={selectedManufacturer}
            onChange={(e) => setSelectedManufacturer(e.target.value)}
            style={{ width: "100%", padding: "8px" }}
            disabled={useLink}
          >
            <option value="" disabled>
              Select a manufacturer
            </option>
            {manufacturers.map((mfg, idx) => (
              <option key={idx} value={mfg}>
                {mfg}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label style={{ display: "block", marginBottom: "5px" }}>
            Device *
          </label>
          <select
            required
            value={selectedDevice}
            onChange={(e) => setSelectedDevice(e.target.value)}
            disabled={!selectedManufacturer || devices.length === 0}
            style={{ width: "100%", padding: "8px" }}
            disabled={useLink}
          >
            <option value="" disabled>
              {!selectedManufacturer
                ? "Select manufacturer first"
                : isFetchingDevices
                  ? "Fetching Devices"
                  : "Select a device"}
            </option>
            {devices.map((dev, idx) => (
              <option key={idx} value={dev}>
                {dev}
              </option>
            ))}
          </select>
        </div>

        {/* --- OPTIONAL FIELDS --- */}
        <hr style={{ width: "100%", border: "1px solid #ccc" }} />

        <div>
          <label
            style={{
              display: "flex",
              alignItems: "center",
              gap: "8px",
              cursor: "pointer",
            }}
          >
            <input
              type="checkbox"
              checked={useLink}
              onChange={(e) => setUseLink(e.target.checked)}
            />
            Link (Optional)
          </label>
        </div>

        {/* Render radio buttons and input ONLY if checkbox is checked */}
        {useLink && (
          <div
            style={{
              padding: "10px",
              backgroundColor: "#f9f9f9",
              borderRadius: "5px",
            }}
          >
            <div style={{ marginBottom: "10px", display: "flex", gap: "15px" }}>
              <label style={{ cursor: "pointer" }}>
                <input
                  type="radio"
                  name="source"
                  value="phonedb"
                  checked={linkSource === "phonedb"}
                  onChange={(e) => setLinkSource(e.target.value)}
                  style={{ marginRight: "5px" }}
                />
                PhoneDB
              </label>
              <label style={{ cursor: "pointer" }}>
                <input
                  type="radio"
                  name="source"
                  value="gsmarena"
                  checked={linkSource === "gsmarena"}
                  onChange={(e) => setLinkSource(e.target.value)}
                  style={{ marginRight: "5px" }}
                />
                GSMArena
              </label>
            </div>

            <div>
              <input
                type="text"
                placeholder="oneplus/apple/xiaomi etc..."
                value={selectedManufacturer}
                onChange={(e) => setSelectedManufacturer(e.target.value)}
                style={{ width: "100%", padding: "8px" }}
              />
              <input
                type="text"
                placeholder="oneplus 15 R/ 17 Ultra/ iPhone 17 etc..."
                value={selectedDevice}
                onChange={(e) => setSelectedDevice(e.target.value)}
                style={{ width: "100%", padding: "8px" }}
              />
              <input
                type="url"
                placeholder="https://phonedb.net... or https://gsmarena.com..."
                value={linkUrl}
                onChange={(e) => setLinkUrl(e.target.value)}
                style={{ width: "100%", padding: "8px" }}
              />
            </div>
          </div>
        )}

        <button
          type="submit"
          disabled={isLoading}
          style={{
            padding: "10px",
            backgroundColor: "#0056b3",
            color: "white",
            border: "none",
            borderRadius: "4px",
            cursor: "pointer",
          }}
        >
          {isLoading ? "Fetching..." : "Submit and Fetch Data"}
        </button>
      </form>

      {/* --- HUGE BOX FOR JSON OUTPUT --- */}
      <div style={{ marginTop: "30px" }}>
        <h3>JSON Output</h3>
        <pre
          style={{
            width: "100%",
            height: "400px",
            padding: "15px",
            backgroundColor: "#1e1e1e",
            color: "#d4d4d4",
            borderRadius: "5px",
            overflow: "auto",
            boxSizing: "border-box",
          }}
        >
          {jsonOutput
            ? JSON.stringify(jsonOutput, null, 2)
            : "// Output will appear here after fetching"}
        </pre>
      </div>
    </div>
  );
}
