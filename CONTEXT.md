# QuickEval Domain

QuickEval captures independent human evaluations of business Agents and turns quality problems found in evaluations or production usage into traceable Badcases.

## Evaluation Structure

**Evaluation Target（评测对象）**:
A business Agent product whose quality is evaluated, such as the cloud marketplace assistant or intelligent procurement Agent. Its release version and runtime environment are observations captured during an evaluation, not separate targets.
_Avoid_: Agent type, application

**Scenario（评测场景）**:
A coherent business capability evaluated for exactly one Evaluation Target. It is the required classification boundary for Datasets and Badcases, and its target ownership becomes immutable once historical evaluation or Badcase data exists.
_Avoid_: Category, project

**Dataset（评测集）**:
A named, evolving collection of Evaluation Cases within one Scenario. A Dataset is the stable identity across its versions, and its Scenario ownership becomes immutable once a version is published.
_Avoid_: Task set, question bank

**Dataset Version（评测集版本）**:
An immutable published snapshot of a Dataset's Evaluation Cases. A Dataset has at most one editable draft; published versions receive permanent sequential numbers, while archived versions remain historical but cannot start new evaluations.
_Avoid_: Dataset snapshot, copy

**Evaluation Case（评测用例）**:
A single user question or task instruction to be exercised against an Agent. It retains its logical identity across Dataset Versions while each version preserves its own content snapshot; reference answers, operating steps, and judging guidance are optional context.
_Avoid_: Question, test point

**Case Tag（用例标签）**:
A Scenario-specific classification used to organize and filter Evaluation Cases by business content. It is distinct from an Issue Tag, and its assigned identity and displayed name are preserved with the case content in a Dataset Version.
_Avoid_: Issue tag, Badcase category

## Evaluation Execution

**Agent Observation（Agent现场信息）**:
The Agent release identifier, runtime environment, and optional configuration context observed during an Evaluation Run or real business Badcase. It is captured as evidence rather than maintained as release master data.
_Avoid_: Agent version entity, release catalog

**Evaluation Run（评测记录）**:
One evaluator's independent attempt against one Dataset Version, including the observed Agent release, runtime environment, and optional purpose or notes. A run that has been completed or has produced a Badcase is retained; completion includes it in formal summaries, reopening removes it until completion, and voiding is terminal. Repeated runs by the same evaluator are distinct records.
_Avoid_: Evaluation task, batch, assignment

**Case Result（用例结果）**:
The evaluator's state, evidence, and judgment for one Evaluation Case in one Evaluation Run. Every case receives a pending result when a run starts; it becomes evaluated with answer evidence or skipped with a reason, and may contain screenshots, an optional five-point score, or commentary.
_Avoid_: Answer, score record

**Skipped Result（跳过结果）**:
A Case Result that could not be exercised and records a required reason. It is excluded from score and Badcase-rate calculations.
_Avoid_: Missing result, incomplete result

## Quality Issues

**Badcase**:
A traceable record of an observed Agent quality problem and the sole source of truth for whether a Case Result is a Badcase. It originates either from a Case Result or from real business usage, always belongs to a Scenario, and can be invalidated independently of its processing status; one Case Result can originate at most one Badcase.
_Avoid_: Bug, failure record

**Issue Tag（问题标签）**:
A globally managed classification that can be attached to multiple Badcases. Disabled tags remain attached to historical Badcases, while renaming a tag intentionally updates the current classification shown across history.
_Avoid_: Case tag, category

**Badcase Activity（处理记录）**:
An immutable timeline entry recording a Badcase status change, assignee change, or processing note together with its actor and time.
_Avoid_: Editable comment, operation log
