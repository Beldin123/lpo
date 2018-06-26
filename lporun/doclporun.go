/* 

Executable provides examples of lpo use and exerciser for exported functions.

SUMMARY


This executable provides examples of how the lpo package can be used to solve 
linear programming (LP) and mixed integer linear programming (MIP) problems
via Cplex and the callable C routines.
It also provides an exerciser to individually call and test each function exported by lpo
and the companion gpx packages.

The user must select one of the provided options to perform the desired task.
The options are grouped into a set of commands which illustrate main lpo functionality.
Several options have been included to exercise the gpx package, which is imported
by lpo, and contains all functions that directly access the Cplex callable C routines.

The options available from the main menu are:

    0 - exit program
    1 - using SolveProb to read, reduce, and solve a model via lpo
    2 - using Cplex to read and solve a problem directly (bypass most of lpo)
    3 - using ReduceMatrix to reduce but not solve a problem
    4 - read MPS file (needed by other functions called at a later time)
    5 - write MPS file (useful if user-written functions populated lpo structures)
    6 - initialize lpo structures (needed in conjunction with lpo function exerciser)
    7 - show lpo input data structures
    8 - show lpo solution provided by Cplex
    9 - initialize gpx structures (needed in conjunction with gpx function exerciser)
   10 - write gpx file to be read by gpxrun in the companion gpx package
   11 - show gpx input data structures
   12 - show gpx solution data structures
	

In addition to the commands, the main menu also includes the following toggles:	

    m - toggle main menu display (to reduce clutter)
    c - toggle custom environment (to reduce typing when entering file names)
    s - toggle lpo function exerciser (to enable access to exported lpo functions)
    g - toggle gpx function exerciser (to enable access to exported gpx functions)

To select an option, enter the corresponding letter or number when prompted.


MAIN COMMANDS

The main command options are displayed at all times and are as follows. To redisplay
the available options, enter a blank line or any other "unsupported" option.

Exit

This option is used to terminate execution of the program. This option is displayed
as part of the command prompt and is not included in the lists showing other options.

Using SolveProb

This option illustrates how to load a model into lpo, reduce the problem size,
translate the lpo data structures to the corresponding gpx data structures,
use gpx to call the Cplex C functions to solve the problem and provide a solution,
reconstitute the problem into its original form, and present the results to the
user. All of the work is done by the main SolveProb function, and the example
consists of populating the fields of the control structure passed to SolveProb
and interpretting the results that it returns.

The first prompt the user must answer is the source from which the model is to be
read. This can be the name of an MPS file, or can be left empty (carriage return at
the prompt) if the model is already loaded into the internal data structures as
a result of an earlier operation, and should be taken from there.

The next set of prompts allows the user to specify the file names for storing
the Cplex solution (xml), reduced matrix (MPS), and pre-solve operations (text).
These files are optional, and if the name is not entered, the file will not be written.
If the custom environment is enabled, the prompt for these file names will not
appear, it will be assumed that all of them are needed, and the name will be based
on the file name provided with the appropriate prefix added (see custom environment
section for details).

The next prompt allows the user to specify which matrix-reduction operations to
apply, and whether to solve the problem. The high-level options are "all" (apply all
reductions and solve the problem), "none" (don't reduce anything and don't solve),
or blank (carriage-return) to set each flag independently. The user must explicitly
enter "Y" at each of the prompts to set the corresponding to "true", since any other
response to the prompt will leave the flag in its default "false" state.

After all prompts have been answered, the populated control structure is passed
to SolveProb, and the requested actions are performed. The main functions that
are called are as follows:
 	
   ReadMpsFile        - If the file name was given, populate the input structures,
                        otherwise use the structures directly. If file not specified
                        and the structures are empty, return an error.
   ReduceMatrix       - Perform the matrix-reduction operations requested.
   WriteMpsFile       - If requested, save the reduced matrix to file.
   WritePsopFile      - If requested, save the pre-solve operations to file.
   CplexCreateProb    - Initialize the Cplex environment and populate the gpx structures.

   if the problem is a MIP
   gpx.MipOpt         - Use gpx to instruct Cplex to solve as a MIP.
   gpx.GetMipSolution - Obtain the MIP solution from Cplex via the gpx data structures.

   otherwise treat as an LP
   gpx.LpOpt          - Use gpx to instruct Cplex to solve as an LP.
   gpx.GetSolution    - Obtain the LP solution from Cplex via the gpx data structures.

   gpx.SolWrite       - If requested, save the Cplex solution to file.
   gpx.CloseCplex     - Close and clean up the Cplex environment.
   misc. lpo func.    - Use miscellaneous internal lpo functions to merge gpx solution
                        structures with PSOP operations list, populate the lpo
                        solution data structure, and pass it back to the caller
                        via the argument list.

Although not part of the SolveProb function, this example then calls a separate
function to process the solution and display it to the user. The "show solution" 
is available as a separate item in the options menu.

Using Cplex functions

This option illustrates how lpo can be used to have Cplex solve a problem while
bypassing most of lpo and gpx functionality. Here, Cplex is given the name of an
MPS file which defines the model and serves as input, and writes the solution 
to a different file as the output. No matrix reduction or population of lpo or
gpx data structures is done in this case.

The functions which are used in this example are:

   CplexSolveMps  - Read the MPS file specified, solve it, and create a solution file.
   CplexParseSoln - Parse the solution file provided by Cplex and populate the
                    lpo.CplexSoln data structure.

The value of the objective function is displayed as part of this example. No
other fields of the solution data structure are displayed, or used in any other
aspect by lpo, but remain available for those users wishing to access them.

Using ReduceMatrix

This example is a subset of the "Using SolveProb" example. The model must be loaded
into the internal data structures, most likely by an earlier call to ReadMpsFile,
TransFromGpx if converting from gpx data structures, or some other similar
mechanism. The user is prompted to specify which matrix-reduction operations are
to be performed using the same set of questions requiring "Y" or "N" responses,
the problem is reduced, and the system is left in this state. The user may then
perform additional operations by independently calling other lpo or gpx functions
as needed.

Read MPS file

This option uses the ReadMpsFile function to populate the internal lpo data structures
from an MPS file. Although this single function is included in the lpo function
exerciser, it is important enough to be included in the main menu.

Write MPS file

Similarly, this option consists of the WriteMpsFile function which is also considered
important enough to be included in the main menu.

Initialize lpo structures

This option initializes all data structures used in this program. It is more
thorough than InitModel, which only initializes the input data structures. This
option is intended to be used in conjunction with the lpo and/or gpx function
exercisers.

Show lpo input

This option shows the lpo input data structures in their raw form. It is not "pretty",
but displays all fields of the various lists, and is useful when exercising other
functions (e.g. DelRow or DelCol). To display a prettier version of the model,
please use one of the other "Print" functions provided for this purpose.

Show lpo solution

This option shows the lpo solution data structures. It is intended to be used
in conjunction with the function exerciser.

Initialize gpx structures

The gpx package is a key component of lpo, and a function exerciser has been provided
for this package as well. This option is intended to initialize the gpx data structures
so that the functions in both packages can be used.

Show gpx input

This option shows the gpx input data structures. It is useful when exercising
the gpx component, which acts as the interface between lpo and Cplex.

Show gpx solution

This option is used to display the solution provided by Cplex. It is useful when
running individual gpx functions which do not automatically show the solution when
it is obtained.

TOGGLES

This section describes the toggles which control program behaviour. The variables
which control the toggles and their default state are:

   var mainMenuOn bool = true    // Flag for main lpo function display
   var lpoMenuOn  bool = false   // Flag for enabling lpo functions   
   var gpxMenuOn  bool = false   // Flag for enabling gpx functions   
   var custEnvOn  bool = false   // Flag for enabling custom paths and names

Toggle main menu display

This toggle turns the display of the main menu on or off. It is intended to reduce
the clutter, particularly if the exercisers are turned on. It affects the display
only, and even if not visible, the options of the main menu remain available at
all times.

Toggle lpo function exerciser

This toggle is used to enable or disable the options which are available to exercise
individual lpo functions. By default, the lpo function exerciser is disabled.

Toggle gpx function exerciser

This toggle is used to enable or disable the options which are available to exercise
individual gpx functions. By default, the gpx function exerciser is disabled.

Toggle for custom environment

This toggle controls how file names are handled by this program. If all files are
located in the same directory and if all files have the same extension, this
option reduces the amount of typing needed to answer various prompts. By default,
the custom environment is disabled and the full file name (including path and
extension) must be specified.

If custom environment is enabled (variable set to "true"), the directory name is
added as a prefix to the base file name, the extension is added as a suffix
to that name, and any "family" of files (e.g. Cplex output, PSOP file, etc.) is based
on the core name input by the user but with a prefix added to it. The default
settings are:

  var dSrcDev       string = "D:/Docs/LP/Data/"  // Development source data dir
  var fPrefCplexOut string = "cp_"               // Prefix for Cplex solution xml files  
  var fPrefRdcMps   string = "r_"                // Prefix for MPS file storing reduced matrix
  var fPrefPsopOut  string = "psop_"             // Prefix for file storing data removed during PSOP
  var fExtension    string = ".txt"              // Extension of source data files in development dir.  

Caution is advised if using a custom environment.

LPO FUNCTION EXERCISER

This section lists the options used to exercise individual gpx functions. Please
refer to the main documentation for details on function input, output, and behaviour.

Care must be taken that the required data structures have been correctly initialized 
and populated, and that the functions are not called out of sequence. The list of 
available functions, listed in alphabetical order, and some things to watch out for, 
are listed below.

 21 - AdjustModel      - Do post-processing after data structures are populated.
 22 - CalcConViolation - Calculate the constraint violation for a given point.
 23 - CalcLhs          - Calculate the LHS for a given point.
 24 - CplexCreateProb  - Initialize Cplex environment and convert to gpx structures.
 25 - CplexParseSoln   - Parse Cplex xml solution file into internal structures.
 26 - CplexSolveMps    - Have Cplex solve the problem defined in the MPS file.
 27 - DelCol           - Delete a specific column from the lpo columns list.
 28 - DelRow           - Delete a specific row from the lpo rows list.
 29 - GetLogLevel      - Get the current log level.
 30 - GetStatistics    - Get the model statistics.
 31 - GetTempDirPath   - Get the current path of the temp directory.
 32 - InitModel        - Initialize the lpo input data structures.
 33 - PrintCol         - Prints the rows in which the column, specified by its index, occurs.
 34 - PrintModel       - Prints the model in equation format.
 35 - PrintRhs         - Prints the RHS of all constraints.
 36 - PrintRow         - Prints the row, specified by its index, in equation format.
 37 - PrintStatistics  - Prints the model statistics.
 38 - ReadMpsFile      - Reads MPS file and populates internal data structures.
 39 - ReduceMatrix     - Performs the matrix-reduction operations specified.
 40 - ScaleRows        - Performs row scaling on the entire model.
 41 - SetLogLevel      - Sets the log level to the value specified.
 42 - SetTempDirPath   - Sets the temp dir location to the path specified.
 43 - SolveProb        - Reduces and solves the model in the MPS file or data structures.
 44 - TightenBounds    - Tightens the bounds on the constraints of the model.
 45 - TransFromGpx     - Populates lpo data structures from the gpx data structures.
 46 - TransToGpx       - Populates gpx data structures from the lpo data structures.
 47 - WriteMpsFile     - Writes the model to an MPS file.
 48 - WritePsopFile    - Writes the pre-solve operations (PSOP) to a text file.

GPX FUNCTION EXERCISER

This section lists the options used to exercise individual gpx functions. The same
functionality is available in an executable included with the gpx package, but has
been included here for convenience. Please refer to the main documentation for 
details on function input, output, and behaviour.

Care must be taken that the required data structures have been correctly initialized 
and populated, and that the functions are not called out of sequence. The list of 
available functions, listed in alphabetical order, and some things to watch out for, 
are listed below.

 61 - ChgCoefList     - Sets non-zero coefficients, must be used after NewCols and NewRows.
 62 - ChgObjSen       - Sets problem to be treated as "maximize" or "minimize".
 63 - ChgProbName     - Sets the problem name.
 64 - CloseCplex      - Cleans up and closed the Cplex environment, must be called last.
 65 - CreateProb      - Initializes the Cplex environment, must be called first.
 66 - GetColName      - Creates the column solution list of the correct size and 
                        populates it with the column names.
 67 - GetMipSolution  - Creates and populates solution structures for MIP problem.
 68 - GetNumCols      - Gets the number of columns in the problem.
 69 - GetNumRows      - Gets the number of rows in the problem.
 70 - GetObjVal       - Gets the obj. func. value, assumes problem has been solved.
 71 - GetRowName      - Creates the row solution list of the correct size and populates 
                        it with the row names.
 72 - GetSlack        - Adds slack values to row solution list, which must exist.
 73 - GetSolution     - Creates and populates solution structures for LP problem.
 74 - GetX            - Adds values to column solution list, which must exist.
 75 - LpOpt           - Optimizes an LP loaded into Cplex.
 76 - MipOpt          - Optimizes a MIP loaded into Cplex.
 77 - NewCols         - Creates new columns in Cplex from the internal data structures.
 78 - NewRows         - Creates new rows in Cplex from the internal data structures.
 79 - OutputToScreen  - Specifies whether Cplex should display output to screen or not.
 80 - ReadCopyProb    - Populates the problem in Cplex directly from the file specified.
 81 - SolWrite        - Writes the Cplex solution to a file.
 82 - WriteProb       - Writes the problem loaded into Cplex to a file using the
                        format specified.



*/
package main