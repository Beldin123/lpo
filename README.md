Package LPO provides a Go language suite of tools for Linear Programming (LP) and Mixed-Integer Linear Programming (MILP). It is intended for two sets of users: (i) researchers working on LP/MILP algorithms, and (ii) users wanting easy Go access to the well-known Cplex solver. Some of the main functions include:

•	Ability to read model files in MPS format, or to create models directly,
•	Model presolving,
•	Evaluating constraints and points,
•	Solving models via submission to the Cplex solver.

LPO indirectly makes use of the callable C functions available in the Cplex solver by using the auxiliary GPX package. GPX functionality is covered in a separate document.
