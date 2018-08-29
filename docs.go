// 01   July  5, 2018   Initial version uploaded to github
// 02   July 12, 2018   Modified with comments from J. Chinneck
// 03   Aug. 29, 2018   Added support for Coin-OR


/*
Package lpo ("linear programming object") provides a suite of Go language tools for 
Linear Programming (LP) and Mixed-Integer Linear Programming (MILP). It is intended 
for two sets of users: (i) researchers working on LP/MILP algorithms, and (ii) 
users wanting easy Go access to the well-known Cplex or Coin-OR solver.

Some of the main functions include:
	- ability to read model files in MPS format, or create models directly
	- model presolving
	- evaluating constraints and points
	- solving models via submissions to the solver

The separate Go language package gpx ("go-Cplex") is used by lpo to interact with
the Cplex solver. Package gpx provides Go language wrappers for several of the most useful
function in the Cplex C language callable library.

Presolving

Package lpo implements some presolving techniques to reduce the size 
of a model. The presolver techniques, described by Andersen,
are available at https://www.researchgate.net/publication/220589130_Presolving_in_linear_programming. 

The presolving algorithms supported by lpo at this time include:

	- removing non-binding constraints
	- removing empty rows              (constraint has no variables)
	- removing row singletons          (constraints that have a single variable)
	- removing fixed variables         (upper bound equals the lower bound)
	- removing free column singletons  (unbounded variable present only in the objective function)

You can control which of these presolving methods are invoked
by setting the appropriate boolean flags and specifying the number of iterations
to be performed. The configurable parameters are:

    type PsCtrl struct {
        FileInMps        string  // Full path for MPS file containing input
        FileOutSoln      string  // Path path for Cplex output file, or "" for default
        FileOutMpsRdcd   string  // Reduced MPS output file, or "" for none
        FileOutPsop      string  // Output file of pre-solve operations, or "" for none
        MaxIter          int     // Maximum iterations for lpo
        DelRowNonbinding bool    // Controls if non-binding rows are removed
        DelRowSingleton  bool    // Controls if row singletons are removed
        DelColSingleton  bool    // Controls if column singletons are removed
        DelFixedVars     bool    // Controls if fixed variables are removed
        RunSolver        bool    // Controls if problem is to be solved 		
    }

Additional reductions will be included in future enhancements.

Creating Model Files

Models can be created in 4 ways:

  - Read in from files in MPS format.
  - Created via functions in the gpx object, then written via Cplex as an MPS file
    for input into lpo.
  - Created via functions in the gpx object then transferred directly into lpo.
  - Created directly using the data structures in lpo.

Interacting with Cplex

Models can be passed to Cplex for manipulation or solution. There are three ways to do this:

  - Once a model is created in lpo, use CplexCreateProb() to transfer it to gpx.
    Use the functions in gpx to solve the model.
  - Use CplexSolveMps() to instruct Cplex to read an MPS file and solve it.
  - Once a model is created in lpo, use CplexSolveProb() to instruct Cplex to solve it.
    Interaction with gpx and Cplex is transparent to the user.

The CplexSolveProb function lets you set all relevant fields in the control data 
structure and have lpo read the model, reduce the size, solve it via Cplex, 
and return the results for additional processing. For example, the code could 
include the following:

  var ctrl   lpo.PsCtrl
  var result lpo.PsSoln
  ...
  // Initialize the control structure.
  ctrl.FileInputMps     = "C:/Data/LP/myModel.txt"
  ctrl.DelRowNonBinding = true
  ctrl.DelRowSingleton  = false
  ...

  // Solve the problem. Return if an error condition is detected.	
  if err := lpo.CplexSolveProb(ctrl, &result); err != nil {
    fmt.Printf("lpo returned the following error: %s\n", err)
    return
  }
  ...
	
The PsSoln structure expands the results provided by Cplex to include
constraints and variables that were removed by the presolver, thus providing the
complete set of results to match the original model. Some parameters (e.g. Slack, 
Dual, ReducedCost) are calculated by Cplex and returned by lpo under some, but not
all conditions. The values are provided on a "best effort" basis. The Status field
is reserved for future use, and is always set to "NA".

Users who do not wish to reduce and solve the problem as described above have the
option of calling individual functions to only perform a specific task. Those
functions are listed and described in the following sections.

Interacting with Coin-OR

Models can be passed to Coin-OR for manipulation or solution. There are two ways to do this:

  - Use CoinSolveMps() to instruct Coin-OR to read an MPS file and solve it.
  - Once a model is created in lpo, use CoinSolveProb() to instruct Coin-OR to solve it.
    Interaction with Coin-OR is transparent to the user.

The CoinSolveProb function lets you set all relevant fields in the control data 
structure and have lpo read the model, reduce the size, solve it via Coin-OR, 
and return the results for additional processing. For example, the code could 
include the following:

  var ctrl   lpo.PsCtrl
  var result lpo.PsSoln
  ...
  // Initialize the control structure.
  ctrl.FileInputMps     = "C:/Data/LP/myModel.txt"
  ctrl.DelRowNonBinding = true
  ctrl.DelRowSingleton  = false
  ...

  // Solve the problem. Return if an error condition is detected.	
  if err := lpo.CoinSolveProb(ctrl, &result); err != nil {
    fmt.Printf("lpo returned the following error: %s\n", err)
    return
  }
  ...
	
The PsSoln structure expands the results provided by Coin-OR to include
constraints and variables that were removed by the presolver, thus providing the
complete set of results to match the original model. Some parameters (e.g. Slack, 
Dual, ReducedCost) are calculated by Coin-OR and returned by lpo under some, but not
all conditions. The values are provided on a "best effort" basis. The Status field
is reserved for future use, and is always set to "NA".

Users who do not wish to reduce and solve the problem as described above have the
option of calling individual functions to only perform a specific task. Those
functions are listed and described in the following sections.

Interacting with Other Solvers

Models can be passed to an external solver other than Cplex or Coin-OR. In this case, the 
"RunSolver" flag is set to false, and other functions are used to, for example, 
write the reduced model to a file so it may be processed by the solver. 
Algorithmic researchers can write their own experimental solvers and simply use 
the lpo functions to read or create a model, to presolve it, to evaluate constraints 
at a point, etc. 

Additional Values Calculated by Solvers

The ReducedCost value associated with variables and the Pi, Slack, and Dual values
associated with constraints are provided by the solver and by lpo only under specific
conditions on a "best effort" basis. As a result, these values should not be 
considered as "reliable". The conditions under which the values are provided are
as follows:

  ReducedCost - model is LP and variable was processed by Cplex or Coin-OR
  Pi          - model is LP and constraint was processed by Cplex
  Slack       - model is LP or MILP and constraint was processed by Cplex
  Dual        - model is LP and constraint was processed by Coin-OR

Variables and constraints that were removed during preprocessing will not have
these values, regardless of which model and solver was used. Future enhancements
are planned to address this deficiency.

Tutorial and Function Exerciser

The executable provided with the package illustrates how the lpo package can be
used.


*/
package lpo
