---
title: Date Intelligence
description: Why handling dates in Git is harder than it looks.
---

Standups are entirely dependent on time. "Yesterday" and "Today". 

But extracting the correct concept of "Yesterday" from a Git repository requires careful date handling.

## The Rebase Problem: `%aI` vs `%cI`

Every git commit actually has two dates:
1. **Author Date (`%aI`):** When the commit was originally written.
2. **Commit Date (`%cI`):** When the commit was applied to the current tree.

If you write a commit on Monday, but then `git rebase` your branch on Wednesday to fix merge conflicts before merging, the Author Date remains Monday. 

If a standup tool relies on the Author Date, your Wednesday standup will be completely empty, because Git thinks that work was done on Monday.

`git-brief` strictly parses the **Commit Date (`%cI`)**. When you rebase or squash, the Commit Date is updated, ensuring you get accurate credit for the day you actually integrated the work.

## The Weekend Look-back

If you run `git brief` on a Tuesday, "Yesterday" means Monday. 

But if you run `git brief` on a Monday, "Yesterday" means Friday (and the weekend). 

`git-brief` has a built-in weekend look-back algorithm. If the current day is Monday, it automatically extends the `--since` argument passed to the Git and GitHub collectors to `3 days ago`. You don't have to pass any manual flags on Monday mornings.
