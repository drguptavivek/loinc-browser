// Code generated from docs/vendor/loinc/fhir.loinc.org-codesystem-search-sample.json. DO NOT EDIT.
// Regenerate by re-running the extraction described in pkg/terminology/properties.go.

package terminology

// CodeSystemProperty is one CodeSystem.property definition, in upstream fhir.loinc.org order.
type CodeSystemProperty struct {
	Code        string
	URI         string
	Description string
	Type        string
}

// CodeSystemFilter is one CodeSystem.filter definition.
type CodeSystemFilter struct {
	Code        string
	Description string
	Operator    []string
	Value       string
}

// codeSystemProperties holds all 83 CodeSystem.property definitions for http://loinc.org,
// in the exact order fhir.loinc.org publishes them (docs/vendor/loinc/fhir.loinc.org-codesystem-search-sample.json).
var codeSystemProperties = []CodeSystemProperty{
	{Code: "parent", URI: "http://hl7.org/fhir/concept-properties#parent", Description: "A parent code in the Component Hierarchy by System", Type: "code"},
	{Code: "child", URI: "http://hl7.org/fhir/concept-properties#child", Description: "A child code in the Component Hierarchy by System", Type: "code"},
	{Code: "COMPONENT", URI: "http://loinc.org/property/COMPONENT", Description: "First major axis-component or analyte: Analyte Name, Analyte sub-class, Challenge", Type: "Coding"},
	{Code: "PROPERTY", URI: "http://loinc.org/property/PROPERTY", Description: "Second major axis-property observed: Kind of Property (also called kind of quantity)", Type: "Coding"},
	{Code: "TIME_ASPCT", URI: "http://loinc.org/property/TIME_ASPCT", Description: "Third major axis-timing of the measurement: Time Aspect (Point or moment in time vs. time interval)", Type: "Coding"},
	{Code: "SYSTEM", URI: "http://loinc.org/property/SYSTEM", Description: "Fourth major axis-type of specimen or system: System (Sample) Type", Type: "Coding"},
	{Code: "SCALE_TYP", URI: "http://loinc.org/property/SCALE_TYP", Description: "Fifth major axis-scale of measurement: Type of Scale", Type: "Coding"},
	{Code: "METHOD_TYP", URI: "http://loinc.org/property/METHOD_TYP", Description: "Sixth major axis-method of measurement: Type of Method", Type: "Coding"},
	{Code: "CLASS", URI: "http://loinc.org/property/CLASS", Description: "An arbitrary classification of terms for grouping related observations together", Type: "Coding"},
	{Code: "VersionLastChanged", URI: "http://loinc.org/property/VersionLastChanged", Description: "The LOINC version number in which the record has last changed. For new records, this field contains the same value as the VersionFirstReleased property.", Type: "string"},
	{Code: "CHNG_TYPE", URI: "http://loinc.org/property/CHNG_TYPE", Description: "DEL = delete (deprecate); ADD = add; PANEL = addition or removal of child elements or change in the conditionality of child elements in the panel or in sub-panels contained by the panel; NAM = change to Analyte/Component (field #2); MAJ = change to name field other than #2 (#3 - #7); MIN = change to field other than name; UND = undelete", Type: "string"},
	{Code: "DefinitionDescription", URI: "http://loinc.org/property/DefinitionDescription", Description: "Narrative text that describes the LOINC term taken as a whole (i.e., taking all of the parts of the term together) or relays information specific to the term, such as the context in which the term was requested or its clinical utility.", Type: "string"},
	{Code: "STATUS", URI: "http://loinc.org/property/STATUS", Description: "Status of the term. Within LOINC, codes with STATUS=DEPRECATED are considered inactive. Current values: ACTIVE, TRIAL, DISCOURAGED, and DEPRECATED", Type: "string"},
	{Code: "CONSUMER_NAME", URI: "http://loinc.org/property/CONSUMER_NAME", Description: "An experimental (beta) consumer friendly name for this item. The intent is to provide a test name that health care consumers will recognize.", Type: "string"},
	{Code: "CLASSTYPE", URI: "http://loinc.org/property/CLASSTYPE", Description: "1=Laboratory class; 2=Clinical class; 3=Claims attachments; 4=Surveys", Type: "string"},
	{Code: "FORMULA", URI: "http://loinc.org/property/FORMULA", Description: "Contains the formula in human readable form, for calculating the value of any measure that is based on an algebraic or other formula except those for which the component expresses the formula. So Sodium/creatinine does not need a formula, but Free T3 index does.", Type: "string"},
	{Code: "EXMPL_ANSWERS", URI: "http://loinc.org/property/EXMPL_ANSWERS", Description: "For some tests and measurements, we have supplied examples of valid answers, such as \u201c1:64\u201d, \u201cnegative @ 1:16\u201d, or \u201c55\u201d.", Type: "string"},
	{Code: "SURVEY_QUEST_TEXT", URI: "http://loinc.org/property/SURVEY_QUEST_TEXT", Description: "Verbatim question from the survey instrument", Type: "string"},
	{Code: "SURVEY_QUEST_SRC", URI: "http://loinc.org/property/SURVEY_QUEST_SRC", Description: "Exact name of the survey instrument and the item/question number", Type: "string"},
	{Code: "UNITSREQUIRED", URI: "http://loinc.org/property/UNITSREQUIRED", Description: "Y/N field that indicates that units are required when this LOINC is included as an OBX segment in a HIPAA attachment", Type: "string"},
	{Code: "RELATEDNAMES2", URI: "http://loinc.org/property/RELATEDNAMES2", Description: "This field was introduced in version 2.05. It contains synonyms for each of the parts of the fully specified LOINC name (component, property, time, system, scale, method).", Type: "string"},
	{Code: "SHORTNAME", URI: "http://loinc.org/property/SHORTNAME", Description: "Introduced in version 2.07, this field contains the short form of the LOINC name and is created via a table-driven algorithmic process. The short name often includes abbreviations and acronyms.", Type: "string"},
	{Code: "ORDER_OBS", URI: "http://loinc.org/property/ORDER_OBS", Description: "Provides users with an idea of the intended use of the term by categorizing it as an order only, observation only, or both", Type: "string"},
	{Code: "HL7_FIELD_SUBFIELD_ID", URI: "http://loinc.org/property/HL7_FIELD_SUBFIELD_ID", Description: "A value in this field means that the content should be delivered in the named field/subfield of the HL7 message. When NULL, the data for this data element should be sent in an OBX segment with this LOINC code stored in OBX-3 and with the value in the OBX-5.", Type: "string"},
	{Code: "EXTERNAL_COPYRIGHT_NOTICE", URI: "http://loinc.org/property/EXTERNAL_COPYRIGHT_NOTICE", Description: "External copyright holders copyright notice for this LOINC code", Type: "string"},
	{Code: "EXAMPLE_UNITS", URI: "http://loinc.org/property/EXAMPLE_UNITS", Description: "This field is populated with a combination of submitters units and units that people have sent us. Its purpose is to show users representative, but not necessarily recommended, units in which data could be sent for this term.", Type: "string"},
	{Code: "LONG_COMMON_NAME", URI: "http://loinc.org/property/LONG_COMMON_NAME", Description: "This field contains the LOINC name in a more readable format than the fully specified name. The long common names have been created via a tabledriven algorithmic process. Most abbreviations and acronyms that are used in the LOINC database have been fully spelled out in English.", Type: "string"},
	{Code: "EXAMPLE_UCUM_UNITS", URI: "http://loinc.org/property/EXAMPLE_UCUM_UNITS", Description: "The Unified Code for Units of Measure (UCUM) is a code system intended to include all units of measures being contemporarily used in international science, engineering, and business. (www.unitsofmeasure.org) This field contains example units of measures for this term expressed as UCUM units.", Type: "string"},
	{Code: "STATUS_REASON", URI: "http://loinc.org/property/STATUS_REASON", Description: "Classification of the reason for concept status. This field will be Null for ACTIVE concepts, and optionally populated for terms in other status where the reason is clear. DEPRECATED or DISCOURAGED terms may take values of: AMBIGUOUS, DUPLICATE, or ERRONEOUS.", Type: "string"},
	{Code: "STATUS_TEXT", URI: "http://loinc.org/property/STATUS_TEXT", Description: "Explanation of concept status in narrative text. This field will be Null for ACTIVE concepts, and optionally populated for terms in other status.", Type: "string"},
	{Code: "CHANGE_REASON_PUBLIC", URI: "http://loinc.org/property/CHANGE_REASON_PUBLIC", Description: "Detailed explanation about special changes to the term over time.", Type: "string"},
	{Code: "COMMON_TEST_RANK", URI: "http://loinc.org/property/COMMON_TEST_RANK", Description: "Ranking of approximately 2000 common tests performed by laboratories in USA.", Type: "string"},
	{Code: "COMMON_ORDER_RANK", URI: "http://loinc.org/property/COMMON_ORDER_RANK", Description: "Ranking of approximately 300 common orders performed by laboratories in USA.", Type: "string"},
	{Code: "HL7_ATTACHMENT_STRUCTURE", URI: "http://loinc.org/property/HL7_ATTACHMENT_STRUCTURE", Description: "This property is populated in collaboration with the HL7 Payer-Provider Exchange (PIE) Work Group (previously called Attachments Work Group) as described in the HL7 Attachment Specification: Supplement to Consolidated CDA Templated Guide.", Type: "string"},
	{Code: "EXTERNAL_COPYRIGHT_LINK", URI: "http://loinc.org/property/EXTERNAL_COPYRIGHT_LINK", Description: "For terms that have a third party copyright, this field is populated with the COPYRIGHT_ID from the Source Organization table (see below). It links an external copyright statement to a term.", Type: "string"},
	{Code: "PanelType", URI: "http://loinc.org/property/PanelType", Description: "For LOINC terms that are panels, this attribute classifies them as a 'Convenience group', 'Organizer', or 'Panel'", Type: "string"},
	{Code: "AskAtOrderEntry", URI: "http://loinc.org/property/AskAtOrderEntry", Description: "A multi-valued, semicolon delimited list of LOINC codes that represent optional Ask at Order Entry (AOE) observations for a clinical observation or laboratory test. A LOINC term in this field may represent a single AOE observation or a panel containing several AOE observations.", Type: "Coding"},
	{Code: "AssociatedObservations", URI: "http://loinc.org/property/AssociatedObservations", Description: "A multi-valued, semicolon delimited list of LOINC codes that represent optional associated observation(s) for a clinical observation or laboratory test. A LOINC term in this field may represent a single associated observation or panel containing several associated observations.", Type: "Coding"},
	{Code: "VersionFirstReleased", URI: "http://loinc.org/property/VersionFirstReleased", Description: "This is the LOINC version number in which this LOINC term was first published.", Type: "string"},
	{Code: "ValidHL7AttachmentRequest", URI: "http://loinc.org/property/ValidHL7AttachmentRequest", Description: "A value of Y in this field indicates that this LOINC code can be sent by a payer as part of an HL7 Attachment request for additional information.", Type: "string"},
	{Code: "DisplayName", URI: "http://loinc.org/property/DisplayName", Description: "A name that is more 'clinician-friendly' compared to the current LOINC Short Name, Long Common Name, and Fully Specified Name. It is created algorithmically from the manually crafted display text for each Part and is generally more concise than the Long Common Name.", Type: "string"},
	{Code: "answer-list", URI: "http://loinc.org/property/answer-list", Description: "An answer list associated with this LOINC code (if there are matching answer lists defined).", Type: "Coding"},
	{Code: "MAP_TO", URI: "http://loinc.org/property/MAP_TO", Description: "A replacement term that is to be used in place of the deprecated or discouraged term.", Type: "Coding"},
	{Code: "analyte", URI: "http://loinc.org/property/analyte", Description: "First sub-part of the Component, i.e., the part of the Component before the first carat", Type: "Coding"},
	{Code: "analyte-core", URI: "http://loinc.org/property/analyte-core", Description: "The primary part of the analyte without the suffix", Type: "Coding"},
	{Code: "analyte-suffix", URI: "http://loinc.org/property/analyte-suffix", Description: "The suffix part of the analyte, if present, e.g., Ab or DNA", Type: "Coding"},
	{Code: "analyte-numerator", URI: "http://loinc.org/property/analyte-numerator", Description: "The numerator part of the analyte, i.e., everything before the slash in analytes that contain a divisor", Type: "Coding"},
	{Code: "analyte-divisor", URI: "http://loinc.org/property/analyte-divisor", Description: "The divisor part of the analyte, if present, i.e., after the slash and before the first carat", Type: "Coding"},
	{Code: "analyte-divisor-suffix", URI: "http://loinc.org/property/analyte-divisor-suffix", Description: "The suffix part of the divisor, if present", Type: "Coding"},
	{Code: "challenge", URI: "http://loinc.org/property/challenge", Description: "Second sub-part of the Component, i.e., after the first carat", Type: "Coding"},
	{Code: "adjustment", URI: "http://loinc.org/property/adjustment", Description: "Third sub-part of the Component, i.e., after the second carat", Type: "Coding"},
	{Code: "count", URI: "http://loinc.org/property/count", Description: "Fourth sub-part of the Component, i.e., after the third carat", Type: "Coding"},
	{Code: "time-core", URI: "http://loinc.org/property/time-core", Description: "The primary part of the Time", Type: "Coding"},
	{Code: "time-modifier", URI: "http://loinc.org/property/time-modifier", Description: "The modifier of the Time value, such as mean or max", Type: "Coding"},
	{Code: "system-core", URI: "http://loinc.org/property/system-core", Description: "The primary part of the System, i.e., without the super system", Type: "Coding"},
	{Code: "super-system", URI: "http://loinc.org/property/super-system", Description: "The super system part of the System, if present. The super system represents the source of the specimen when the source is someone or something other than the patient whose chart the result will be stored in. For example, fetus is the super system for measurements done on obstetric ultrasounds, because the fetus is being measured and that measurement is being recorded in the patient's (mother's) chart.", Type: "Coding"},
	{Code: "analyte-gene", URI: "http://loinc.org/property/analyte-gene", Description: "The specific gene represented in the analyte", Type: "Coding"},
	{Code: "category", URI: "http://loinc.org/property/category", Description: "A single LOINC term can be assigned one or more categories based on both programmatic and manual tagging. Category properties also utilize LOINC Class Parts.", Type: "Coding"},
	{Code: "search", URI: "http://loinc.org/property/search", Description: "Synonyms, fragments, and other Parts that are linked to a term to enable more encompassing search results.", Type: "Coding"},
	{Code: "rad-modality-modality-type", URI: "http://loinc.org/property/rad-modality-modality-type", Description: "Modality is used to represent the device used to acquire imaging information.", Type: "Coding"},
	{Code: "rad-modality-modality-subtype", URI: "http://loinc.org/property/rad-modality-modality-subtype", Description: "Modality subtype may be optionally included to signify a particularly common or evocative configuration of the modality.", Type: "Coding"},
	{Code: "rad-anatomic-location-region-imaged", URI: "http://loinc.org/property/rad-anatomic-location-region-imaged", Description: "The Anatomic Location Region Imaged attribute is used in two ways: as a coarse-grained descriptor of the area imaged and a grouper for finding related imaging exams; or, it is used just as a grouper.", Type: "Coding"},
	{Code: "rad-anatomic-location-imaging-focus", URI: "http://loinc.org/property/rad-anatomic-location-imaging-focus", Description: "The Anatomic Location Imaging Focus is a more fine-grained descriptor of the specific target structure of an imaging exam. In many areas, the focus should be a specific organ.", Type: "Coding"},
	{Code: "rad-anatomic-location-laterality-presence", URI: "http://loinc.org/property/rad-anatomic-location-laterality-presence", Description: "Radiology Exams that require laterality to be specified in order to be performed are signified with an Anatomic Location Laterality Presence attribute set to 'True'", Type: "Coding"},
	{Code: "rad-anatomic-location-laterality", URI: "http://loinc.org/property/rad-anatomic-location-laterality", Description: "Radiology exam Laterality is specified as one of: Left, Right, Bilateral, Unilateral, Unspecified", Type: "Coding"},
	{Code: "rad-view-aggregation", URI: "http://loinc.org/property/rad-view-aggregation", Description: "Aggregation describes the extent of the imaging performed, whether in quantitative terms (e.g., '3 or more views') or subjective terms (e.g., 'complete').", Type: "Coding"},
	{Code: "rad-view-view-type", URI: "http://loinc.org/property/rad-view-view-type", Description: "View type names specific views, such as 'lateral' or 'AP'.", Type: "Coding"},
	{Code: "rad-maneuver-maneuver-type", URI: "http://loinc.org/property/rad-maneuver-maneuver-type", Description: "Maneuver type indicates an action taken with the goal of elucidating or testing a dynamic aspect of the anatomy.", Type: "Coding"},
	{Code: "rad-timing", URI: "http://loinc.org/property/rad-timing", Description: "The Timing/Existence property used in conjunction with pharmaceutical and maneuver properties. It specifies whether or not the imaging occurs in the presence of the administered pharmaceutical or a maneuver designed to test some dynamic aspect of anatomy or physiology .", Type: "Coding"},
	{Code: "rad-pharmaceutical-substance-given", URI: "http://loinc.org/property/rad-pharmaceutical-substance-given", Description: "The Pharmaceutical Substance Given specifies administered contrast agents, radiopharmaceuticals, medications, or other clinically important agents and challenges during the imaging procedure.", Type: "Coding"},
	{Code: "rad-pharmaceutical-route", URI: "http://loinc.org/property/rad-pharmaceutical-route", Description: "Route specifies the route of administration of the pharmaceutical.", Type: "Coding"},
	{Code: "rad-reason-for-exam", URI: "http://loinc.org/property/rad-reason-for-exam", Description: "Reason for exam is used to describe a clinical indication or a purpose for the study.", Type: "Coding"},
	{Code: "rad-guidance-for-presence", URI: "http://loinc.org/property/rad-guidance-for-presence", Description: "Guidance for.Presence indicates when a procedure is guided by imaging.", Type: "Coding"},
	{Code: "rad-guidance-for-approach", URI: "http://loinc.org/property/rad-guidance-for-approach", Description: "Guidance for.Approach refers to the primary route of access used, such as percutaneous, transcatheter, or transhepatic.", Type: "Coding"},
	{Code: "rad-guidance-for-action", URI: "http://loinc.org/property/rad-guidance-for-action", Description: "Guidance for.Action indicates the intervention performed, such as biopsy, aspiration, or ablation.", Type: "Coding"},
	{Code: "rad-guidance-for-object", URI: "http://loinc.org/property/rad-guidance-for-object", Description: "Guidance for.Object specifies the target of the action, such as mass, abscess or cyst.", Type: "Coding"},
	{Code: "rad-subject", URI: "http://loinc.org/property/rad-subject", Description: "Subject is intended for use when there is a need to distinguish between the patient associated with an imaging study, and the target of the study.", Type: "Coding"},
	{Code: "document-kind", URI: "http://loinc.org/property/document-kind", Description: "Characterizes the general structure of the document at a macro level.", Type: "Coding"},
	{Code: "document-role", URI: "http://loinc.org/property/document-role", Description: "Characterizes the training or professional level of the author of the document, but does not break down to specialty or subspecialty.", Type: "Coding"},
	{Code: "document-setting", URI: "http://loinc.org/property/document-setting", Description: "Setting is a modest extension of CMS\u2019s coarse definition of care settings, such as outpatient, hospital, etc. Setting is not equivalent to location, which typically has more locally defined meanings.", Type: "Coding"},
	{Code: "document-subject-matter-domain", URI: "http://loinc.org/property/document-subject-matter-domain", Description: "Characterizes the clinical domain that is the subject of the document. For example, Internal Medicine, Neurology, Physical Therapy, etc.", Type: "Coding"},
	{Code: "document-type-of-service", URI: "http://loinc.org/property/document-type-of-service", Description: "Characterizes the kind of service or activity provided to/for the patient (or other subject of the service) that is described in the document.", Type: "Coding"},
	{Code: "answers-for", URI: "http://loinc.org/property/answers-for", Description: "A LOINC Code for which this answer list is used.", Type: "Coding"},
}

// codeSystemFilters holds the CodeSystem.filter definitions for http://loinc.org.
var codeSystemFilters = []CodeSystemFilter{
	{Code: "parent", Description: "Allows for the selection of a set of codes based on their appearance in the LOINC Component Hierarchy by System. Parent selects immediate parent only. For example, the code '79190-5' has the parent 'LP379670-5'", Operator: []string{"="}, Value: "A Part code"},
	{Code: "child", Description: "Allows for the selection of a set of codes based on their appearance in the LOINC Component Hierarchy by System. Child selects immediate children only. For example, the code 'LP379670-5' has the child '79190-5'. Only LOINC Parts have children; LOINC codes do not have any children because they are leaf nodes.", Operator: []string{"="}, Value: "A comma separated list of Part or LOINC codes"},
	{Code: "copyright", Description: "Allows for the inclusion or exclusion of LOINC codes that include 3rd party copyright notices. LOINC = only codes with a sole copyright by Regenstrief. 3rdParty = only codes with a 3rd party copyright in addition to the one from Regenstrief", Operator: []string{"="}, Value: "LOINC | 3rdParty"},
}

// propertyByCode looks up a CodeSystem.property definition by its code.
func propertyByCode(code string) (CodeSystemProperty, bool) {
	for _, p := range codeSystemProperties {
		if p.Code == code {
			return p, true
		}
	}
	return CodeSystemProperty{}, false
}

// termStringPropertyColumns lists the Loinc.csv columns (identical spelling to their property
// code) that $lookup step 3 (§4.3) emits as non-blank valueString properties, in Loinc.csv
// column order. It is exactly the "string"-typed entries of codeSystemProperties, excluding
// CONSUMER_NAME: upstream never emits CONSUMER_NAME as a property (the ConsumerName accessory
// file drives the "ConsumerName" designation instead; see fhir_queries.go FHIRConsumerName).
func termStringPropertyColumns() []string {
	columns := make([]string, 0, len(codeSystemProperties))
	for _, p := range codeSystemProperties {
		if p.Type != "string" || p.Code == "CONSUMER_NAME" {
			continue
		}
		columns = append(columns, p.Code)
	}
	return columns
}
