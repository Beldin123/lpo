// 01   July  5, 2018   Initial version uploaded to github
// 02   July 12, 2018   Modified with comments from J. Chinneck

/*
Package lpo ("linear programming object") provides a suite of Go language tools for 
Linear Programming (LP) and Mixed-Integer Linear Programming (MILP). It is intended 
for two sets of users: (i) researchers working on LP/MILP algorithms, and (ii) 
users wanting easy Go access to the well-known Cplex solver.

Some of the main functions include:
	- ability to read model files in MPS format, or create models directly
	- model presolving
	- evaluating constraints and points
	- solving models via submissions to the Cplex solver

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
		FileInputMps      string  // Full path for MPS file containing input
		FileOutputCplex   string  // Path path for Cplex output file, or "" for default
		MaxIter           int     // Maximum presolver iterations
		DelRowNonbinding  bool    // Controls if non-binding rows are removed
		DelRowSingleton   bool    // Controls if row singletons are removed
		DelColSingleton   bool    // Controls if column singletons are removed
		DelFixedVars      bool    // Controls if fixed variables are removed
		RunCplex          bool    // Controls if problem is to be solved by Cplex 		
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
    Use the functions in gpx to solve the mode.
  - Use CplexSolveMps() to instruct Cplex to read an MPS file and solve it.
  - Once a model is created in lpo, use SolveProb() to instruct Cplex to solve it.
    Interaction with gpx and Cplex is transparent to the user.

The SolveProb function lets you set all relevant fields in the control data 
structure and have lpo read the model, reduce
the size, solve it via Cplex, and return the results for additional processing.
For example, the code could include the following:

	var ctrl   lpo.PsCtrl
	var result lpo.PsSoln
	...
	// Initialize the control structure.
	ctrl.FileInputMps     = "C:/Data/LP/myModel.txt"
	ctrl.DelRowNonBinding = true
	ctrl.DelRowSingleton  = false
	...

	// Solve the problem. Return if an error condition is detected.	
	if err := lpo.SolveProb(ctrl, &result); err != nil {
		fmt.Printf("lpo returned the following error: %s\n", err)
		return
	}
	...
	
The PsSoln structure expands the results provided by Cplex to include
constraints and variables that were removed by the presolver, thus providing the
complete set of results to match the original model. Some parameters (e.g. Slack, 
Dual, ReducedCost) can only be calculated by Cplex and are not available for
constraints or variables removed by the presolver. In such cases, the status of 
the parameter is set to "NA", and the parameter value is set to 0.

Users who do not wish to reduce and solve the problem as described above have the
option of calling individual functions to only perform a specific task. Those
functions are listed and described in the following sections.

Interacting with Other Solvers

Models can be passed to an external solver other than Cplex. In this case, the 
"RunCplex" flag is set to false, and other functions are used to, for example, 
write the reduced model to a file so it may be processed by the solver. 
Algorithmic researchers can write their own experimental solvers and simply use 
the lpo functions to read or create a model, to presolve it, to evaluate constraints 
at a point, etc. 

Tutorial and Function Exerciser

The executable provided with the package illustrates how the lpo package can be
used and contains exercisers to allow each lpo and gpx function to be tested independently.
*/
package lpo
