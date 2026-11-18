package invokers

import (
	"bytes"
	"encoding/json"
	"strings"
)

// The build side of the independent review (design §8 C4, §6 brief 3). Two edges of done-invoker, each read-only toward GitHub:
//   - draftPRGate, before the gates: a build that owes the review, on a product lane, does not close while its PR is ready for
//     review. The PR is opened as a draft and `hw done` prints `gh pr ready` only for an approved head.
//   - reviewAfterDelivery, after the report is delivered: `hw review <lane> <task>` dispatches the reviewer, and a failure prints
//     the command for the brainer (review_owed=1 stands in the receipt, so nothing reads the failure as «nothing owed»).

// productLane is the lane's `product_repo` in the lane table.
func (c *Ctx) productLane(project string) bool {
	doc := c.laneTableDoc()
	if doc == nil {
		return false
	}
	lanes, _ := doc["lanes"].(map[string]any)
	v, _ := lanes[project].(map[string]any)
	b, _ := v["product_repo"].(bool)
	return b
}

// draftPRGate refuses a done report of a build that owes the review while its pull request is not a draft. It acts only for a
// product lane and a receipt that says review_owed; `gh` failing (no PR yet, not logged in) is not a refusal: nothing says the PR is
// ready for review.
func (c *Ctx) draftPRGate(receipt, status, project, workdir string, preflight *[]string) error {
	if status != "done" || receiptValue(receipt, "review_owed") == "" || !c.productLane(project) {
		return nil
	}
	var b bytes.Buffer
	rc := c.run(runOpts{stdout: &b}, "sh", "-c", `cd "$1" && exec gh pr view --json isDraft,number`, "sh", workdir)
	if rc != 0 {
		*preflight = append(*preflight, "pr draft: not checked (gh pr view exit "+itoa(rc)+": no PR, or gh is not usable here)")
		return nil
	}
	var pr struct {
		IsDraft *bool `json:"isDraft"`
		Number  int   `json:"number"`
	}
	if json.Unmarshal(b.Bytes(), &pr) != nil || pr.IsDraft == nil {
		*preflight = append(*preflight, "pr draft: not checked (gh printed no isDraft)")
		return nil
	}
	if !*pr.IsDraft {
		return die("this build owes the independent review, and its pull request #%d is ready for review, not a draft. A PR becomes ready only after the reviewer approves its head: convert it with `gh pr ready --undo %d`, then close again. NOTHING was sent or marked.", pr.Number, pr.Number)
	}
	*preflight = append(*preflight, "pr draft: #"+itoa(pr.Number)+" is a draft")
	return nil
}

// KnownAccount is whether hw knows `name` as an account: the names `--account` accepts. It mirrors dsp.accountConfigDirOf in the
// core package (this one cannot import it), and a test there holds the two together.
func KnownAccount(name string) bool { return name == "default" || name == "personal" }

// reviewAfterDelivery runs `hw review <lane> <task>` once the report is delivered: task 1 of a build that owes the review (a
// re-task's re-check is the brainer's `hw next` onto the kept reviewer pane). It never changes the report's outcome.
func (c *Ctx) reviewAfterDelivery(hwBin, receipt, status, project, task string) {
	if status != "done" || c.taskSeq() != "1" || receiptValue(receipt, "review_owed") == "" {
		return
	}
	// `review` is a brainer verb: bin/hw refuses it for an executor (HW_TASK/HW_RUN set), so the invoker, which is hw's own, runs it
	// without them, as qaLedgerRow does for `ledger` (setup/tests/1338).
	var env []string
	for _, kv := range c.environ() {
		if !strings.HasPrefix(kv, "HW_TASK=") && !strings.HasPrefix(kv, "HW_RUN=") {
			env = append(env, kv)
		}
	}
	// the review runs on the account the build ran on (the dispatch's `account` receipt): `hw review`'s own default is the lane's
	args := []string{"review", project, task}
	if account := receiptValue(receipt, "account"); KnownAccount(account) {
		args = append(args, "--account", account)
	} else if account != "" { // the receipt is the executor's to write: a value hw would refuse on its command line is not carried
		c.errf("the receipt names account %q, which hw does not know: the review runs on the lane's own account.\n", account)
	}
	review := "hw " + strings.Join(args, " ")
	if rc := c.run(runOpts{stdout: c.Stdout, stderr: c.Stderr, env: env}, hwBin, args...); rc != 0 {
		c.printf(c.Stdout, "THE REPORT IS DELIVERED AND SAFE, but the independent review was NOT dispatched (hw review exit %d; review_owed=1 stands in the receipt). Brainer: run `%s` once the cause above is fixed.\n", rc, review)
		return
	}
	c.printf(c.Stdout, "independent review dispatched by `%s`.\n", review)
}
