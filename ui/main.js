main();

function formatJSON(object) {
	const string = JSON.stringify(object);
	let outstring = "";
	let tabChar = "  ";
	let tabs = 0;
	for (let i = 0; i < string.length; i++) {
		if (string[i] === "{" || string[i] === "[") {
			tabs++;
			outstring += string[i] + "\n" + tabChar.repeat(tabs);
		} else if (string[i] === "}" || string[i] === "]") {
			tabs--;
			outstring += "\n" + tabChar.repeat(tabs) + string[i];
		} else if (string[i] === ",") {
			outstring += string[i] + "\n" + tabChar.repeat(tabs);
		} else {
			outstring += string[i];
		}
	}
	return outstring;
}

function colorizeJSON(jsonString) {
	const lines = jsonString.split("\n");
	const output = [];
	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		console.log(line);
		if (line.includes(":")) {
			let [prop, val] = line.split('":');
			prop = prop.replace(/"/, "");
			prop = `<span class="prop">${prop}</span>: `;
			if (!val.includes("[")) {
				val = `<span class="val">${val}</span>`;
			}
			output.push(prop + val);
		} else {
			output.push(line);
		}
	}
	return output.join("\n");
}

function topResponseTimes(obj) {
	const array = [];
	for (let i = 0; i < obj.PlaybookMetrics.length; i++) {
		for (let j = 0; j < obj.PlaybookMetrics[i].Metrics.length; j++) {
			array.push(obj.PlaybookMetrics[i].Metrics[j]);
		}
	}
	array.sort(function (a, b) {
		// return a.ResponseTimeMs - b.ResponseTimeMs;
    return b.ResponseTimeMs - a.ResponseTimeMs;
	});

  return array;
}

async function main() {
	const defaultLoadTest = "/ui/test.json";
	const response = await fetch(defaultLoadTest);
	const loadTest = await response.json();

	const outputTextArea = document.getElementById("ot-textarea");

	// const fast = topResponseTimes(loadTest);

	outputTextArea.innerHTML += colorizeJSON(formatJSON(loadTest));
	// outputTextArea.innerHTML += colorizeJSON(formatJSON(fast));
}
