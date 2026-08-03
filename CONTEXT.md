# QuickEval Domain

QuickEval captures independent human evaluations of business Agents and turns quality problems found in evaluations or production usage into traceable Badcases.

**Local Account（本地账号）**:
An account authenticated with a username and password. Administrators create it with the system initial password; it remains password-change-required until its owner sets a new password, and cannot access business work before then.
_Avoid_: Temporary account, passwordless user

## Evaluation Structure

**Evaluation Target（评测对象）**:
A business Agent product whose quality is evaluated, such as the cloud marketplace assistant or intelligent procurement Agent. Its release version and runtime environment are observations captured during an evaluation, not separate targets.
_Avoid_: Agent type, application

**Scenario（评测场景）**:
A target-scoped classification used to organize and analyze Evaluation Cases and Badcases. Scenario coverage may be incomplete, and classification is optional rather than an ownership boundary.
_Avoid_: Category, project

**Dataset（评测集）**:
A named, evolving collection of Evaluation Cases for exactly one Evaluation Target. A Dataset may contain unclassified cases and cases classified into multiple Scenarios.
_Avoid_: Task set, question bank

**Dataset Version（评测集版本）**:
An immutable published snapshot of a Dataset's Evaluation Cases. A Dataset has at most one editable draft; published versions receive permanent sequential numbers, while archived versions remain historical but cannot start new evaluations.
_Avoid_: Dataset snapshot, copy

**Evaluation Case（评测用例）**:
A single user question or task instruction to be exercised against an Agent. It inherits its Evaluation Target from its Dataset, may be classified into a Scenario later, and retains its logical identity across Dataset Versions.
_Avoid_: Question, test point

**Scenario Classification（场景归类）**:
The optional, reviewable assignment of an Evaluation Case or Badcase to a Scenario. Unclassified work remains valid, while automated suggestions are distinct from human-confirmed assignments.
_Avoid_: Scenario ownership, default scenario

**Case Tag（用例标签）**:
A classification used to organize and filter Evaluation Cases by the capability being exercised. A Global Case Tag is available to every Evaluation Target, while a Target Case Tag is available only to its owning Evaluation Target. It is independent from Scenario Classification, distinct from an Issue Tag, and its assigned identity and displayed name are preserved with the case content in a Dataset Version.
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
A traceable record of an observed Agent quality problem and the sole source of truth for whether a Case Result is a Badcase. Its Concrete Problem is the primary business fact; it may originate from a Case Result or real business usage, be classified into a Scenario later, and be invalidated independently of its processing status.
_Avoid_: Bug, failure record

**Concrete Problem（具体问题）**:
A concrete account of the observed behavior, judgment basis, or location clue. It is required when a Case Result is marked as a Badcase.
_Avoid_: Badcase title, comment

**Badcase Title（Badcase 标题）**:
A concise display summary used to identify a Badcase in lists and links. It is entered for a standalone business registration and may be derived from the Concrete Problem for an evaluation-origin Badcase.
_Avoid_: Concrete problem

**Issue Tag（问题标签）**:
An optional, globally managed classification for aggregation and trend analysis across Badcases. A Badcase may remain unclassified or be classified later; disabled tags remain attached to history, while renaming intentionally updates the current classification shown across history.
_Avoid_: Case tag, required problem type

**Badcase Activity（处理记录）**:
An immutable timeline entry recording a Badcase status change, assignee change, or processing note together with its actor and time.
_Avoid_: Editable comment, operation log
