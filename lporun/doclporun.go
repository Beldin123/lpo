/* 

Executable demonstrates how the key exported lpo functions are used.

SUMMARY


This executable provides examples of how the lpo package can be used to solve 
linear programming (LP) and mixed integer linear programming (MILP) problems
via Coin-OR or Cplex solvers.

The user must select one of the numbers corresponding to the command they wish
to execute from the following list:

  0 - EXIT program
  1 - read, print, and reduce model without solving
  2 - solve small LP problem using Coin-OR CLP solver
  3 - solve small MILP problem using Coin-OR CBC solver
  4 - solve large problem using Cplex and gpx
  5 - display lpo solution

This program must executed from the same directory in which the program and the
sample files are located. If the program is executed from a different directory,
or if the sample files reside in a different directory than the executable, the 
following variables must be changed to contain the correct absolute path for each file:

  var inputSmLP   string = "inputSmallLp.txt"    // small LP example MPS file
  var inputSmMILP string = "inputSmallMilp.txt"  // small MILP example MPS file      
  var inputLg     string = "inputLargeLP.txt"    // large example MPS file
  var outSoln     string = "soln_file.txt"       // solution output file
  var outPsop     string = "psop_file.txt"       // PSOP output file
  var outRedMtx   string = "rmx_file.txt"        // reduced matrix output file


Exit

This option is used to terminate execution of the program.

Read, print, and reduce model without solving

This option is used to read an MPS file to populate the lpo data structures,
displays the model in equation format, displays the statistics about the model,
reduces the size of the model, prints the reduced model in MPS format to file,
prints the reductions performed to file, but does not solve the problem.

The exported functions which are executed are:

   ReadMpsFile        - Populate the input structures.
   PrintModel         - Print the model in equation format.
   GetStatistics      - Populate the statistics data structure.
   PrintStatistics    - Print the statistics.   
   ReduceMatrix       - Perform the matrix-reduction operations requested.
   WriteMpsFile       - Save the reduced matrix to file.
   WritePsopFile      - Save the pre-solve operations to file.


Solve small LP problem using Coin-OR CLP solver

This option demonstrates how a small LP is solved using the Coin-OR solver. The
example populates the control data structure, passes it to the lpo.CoinSolveProb
function, and processes the solution passed back by this function. All steps, which
include reading the model, deciding which solver to invoke, running the solver,
and processing the results, are performed by this function and are transparent
to the user.

If Coin-OR is not installed, the function will return an error. However, it is not
necessary to exclude any files from the build, as the absence of Coin-OR will not
cause the build to fail, and will not prevent any functionality not associated with
Coin-OR from working.

Solve small MILP problem using Coin-OR CBC solver

This option demonstrates how a small MILP is solved using the Coin-OR solver. The
example populates the control data structure, passes it to the lpo.CoinSolveProb
function, and processes the solution passed back by this function. All steps, which
include reading the model, deciding which solver to invoke, running the solver,
and processing the results, are performed by this function and are transparent
to the user.

If Coin-OR is not installed, the function will return an error. However, it is not
necessary to exclude any files from the build, as the absence of Coin-OR will not
cause the build to fail, and will not prevent any functionality not associated with
Coin-OR from working.

Solve large problem using Cplex and gpx

This option demonstrates how a large LP is solved using the Cplex solver and
callable C libraries accessed via the gpx interface. The example populates the 
control data structure, passes it to the lpo.CplexSolveProb
function, and processes the solution passed back by this function. All steps, which
include reading the model, translating from lpo to gpx structures, running the solver,
and processing the results, are performed by this function and are transparent
to the user.

If gpx is not installed, some files and functions must be removed from the build
in order to avoid build failures and to allow the rest of the program to work
correctly. 

The changes that need to be made are as follows:

In the main lpo directory, exclude file ifgpx.go from being built by uncommenting
the top line to be:

  // +build exclude

In the lporun directory, exclude file utilsgpx.go from being built by uncommenting
the top line to be:
  // +build exclude

In the lporun directory, comment out the line which calls wpCplexSolveProb to
be as follows:

  // Comment out the following line if gpx is not installed and
  // the utilsgpx.go file is excluded from being built.
  // err = wpCplexSolveProb()


Display lpo solution

This option is used to display the solution provided by the solver. Note that the
reduced cost for variables and slack, pi, and dual values for constraints may not
be provided for all combinations of solvers used (Cplex LP, Cplex MILP, Coin-OR LP, or
Coin-OR MILP) and methods of processing (passed to solver or removed during presolve).
These values are provided on a "best effort" basis.

*/
package main