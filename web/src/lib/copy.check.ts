import assert from 'node:assert';
import { asHL7v2CWE, asFHIRCoding, toCSV, type CopyTerm } from './copy.ts';

const term: CopyTerm = { loincNum: '2951-2', longCommonName: 'Sodium [Moles/volume] in Serum or Plasma' };

// HL7 escaping: backslash must escape first, then | ^ & ~
assert.strictEqual(
	asHL7v2CWE({ loincNum: '1', longCommonName: 'a\\b|c^d&e~f' }),
	'1^a\\E\\b\\F\\c\\S\\d\\T\\e\\R\\f^LN'
);
assert.strictEqual(asHL7v2CWE(term), '2951-2^Sodium [Moles/volume] in Serum or Plasma^LN');

// FHIR Coding JSON, with and without version
assert.strictEqual(
	asFHIRCoding(term),
	JSON.stringify({ system: 'http://loinc.org', code: '2951-2', display: term.longCommonName }, null, 2)
);
assert.strictEqual(
	asFHIRCoding(term, '2.82'),
	JSON.stringify(
		{ system: 'http://loinc.org', version: '2.82', code: '2951-2', display: term.longCommonName },
		null,
		2
	)
);

// CSV quoting: comma, quote, newline
assert.strictEqual(
	toCSV([{ a: 'x,y', b: 'has "quote"', c: 'line1\nline2' }], ['a', 'b', 'c']),
	'a,b,c\r\n"x,y","has ""quote""","line1\nline2"'
);
assert.strictEqual(toCSV([{ a: 'plain', b: undefined }], ['a', 'b']), 'a,b\r\nplain,');

console.log('copy.check.ts: all assertions passed');

// Formula injection: a leading = + - @ is neutralised with a quote.
assert.equal(toCSV([{ a: '=HYPERLINK("x")', b: '-5', c: 'ok' }], ['a', 'b', 'c']), 'a,b,c\r\n"\'=HYPERLINK(""x"")",\'-5,ok');
console.log('copy.check.ts: formula guard passed');
