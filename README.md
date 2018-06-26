### Exported Functions

Package LPO provides a Go language suite of tools for Linear Programming (LP) and Mixed-Integer Linear Programming (MILP). It is intended for two sets of users: (i) researchers working on LP/MILP algorithms, and (ii) users wanting easy Go access to the well-known Cplex solver. Some of the main functions include:

•	Ability to read model files in MPS format, or to create models directly,

•	Model presolving,

•	Evaluating constraints and points,

•	Solving models via submission to the Cplex solver.

LPO indirectly makes use of the callable C functions available in the Cplex solver by using the independent GPX package.

### Executable

The lporun subdirectory contains the executable intended as a tutorial demonstrating how lpo and gpx functions are used as well
as exercisers which permit users to independently call and test each function exported by the two packages.
