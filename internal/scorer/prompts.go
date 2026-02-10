package scorer

// EvaluationPrompt is the system prompt used for LLM-as-judge scoring.
// Ported verbatim from the Python score-results.py.
const EvaluationPrompt = `You are a research assistant, evaluating the responses to some exam questions on Kubernetes.

The user submits questions and answers, both the expected answers as well as the actual answers provided by a candidate.

Your task is to evaluate whether the actual answer is correct or not, and then count the number of correct answers. Any single answer may only be correct or incorrect.

Correct means that the answer contains the necessary information. A correct answer is not necessarily identical to the expected answer.

Example input:

---
NO. 2 - Setup & Aliases
QUESTION: How do you enable bash autocompletion for the 'k' alias?
EXPECTED ANSWER: complete -F __start_kubectl k
ACTUAL ANSWER: ` + "```bash" + `
source <(kubectl completion bash)
alias k=kubectl
complete -F __start_kubectl k
` + "```" + `

Example output:

58 out of 100 answers are correct.`
