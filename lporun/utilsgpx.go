// This file contains common routines that are used by gpxrun and lporun.

package main

import (
	"fmt"
	"github.com/Beldin123/gpx"
	"github.com/pkg/errors"
	"strings"
)

// Need to declare gpx variables here to avoid passing them as arguments to the
// wrapper functions as individual wrapper commands are executed.

var gName     string            // gpx input problem name
var gRows   []gpx.InputRow      // gpx input rows
var gCols   []gpx.InputCol      // gpx input cols
var gElem   []gpx.InputElem     // gpx input elems
var gObj    []gpx.InputObjCoef  // gpx input objective function coefficients
var sObjVal   float64           // Solution value of objective function
var sRows   []gpx.SolnRow       // Solution rows provided by gpx
var sCols   []gpx.SolnCol       // Solution columns provided by gpx


//==============================================================================

// wpInitGpx initializes all global input and solution variables. It accepts
// no input and returns no values.
func wpInitGpx() {

	// Initialize all global gpx data structures.
	
	gName   = ""
	gRows   = nil
	gCols   = nil
	gElem   = nil
	sObjVal = 0.0
	sRows   = nil
	sCols   = nil
	
}

//==============================================================================

// wpReadDataFile is a wrapper for ReadCopyProb. It prompts the user to enter
// the file type and file name and calls the function to read the file. In
// case of failure, it returns an error.
func wpReadDataFile() error {
	var userString string   // input provided by user
	var fileType   string   // type of file to be read
	var fileName   string   // name of file to be read
	var err        error    // error returned from functions called
	
	fmt.Printf("Enter source file type (LP|MPS|SAV): ")
	fmt.Scanln(&userString)
	fileType = strings.ToUpper(userString)

	switch fileType {

	case "LP", "MPS", "SAV":
		fmt.Printf("Enter source file name: ")
		fmt.Scanln(&fileName)
		if custEnvOn {
			fileName = dSrcDev + fileName + fExtension
		}

		fmt.Printf("\n")
				
		if err = gpx.ReadCopyProb(fileName, fileType); err != nil {
			return errors.Wrap(err, "Open MPS file failed")
		} 
		
	default:
		return errors.Wrapf(err, "Unsupported input file type: %s\n", userString)	
	} // end switch on file type string

	return nil	
}

//==============================================================================

// wpWriteProb gives the user the ability to save the model to a file using any
// of the formats supported by Cplex. The function executes an infinite loop,
// which users must explicitly exit, to allow users the chance to save multiple 
// different files. The function accepts no input and returns no values.
func wpWriteProb() {
	var userString string  // input provided by user
	var fileName   string  // file name
	var fileType   string  // file type
	var err        error   // error returned by functions called
	
	for {
		userString = ""
		fmt.Printf("\nFile types are:\n")
		fmt.Printf("QUIT - done with files   SAV - binary matrix and basis file\n")
		fmt.Printf("MPS  - MPS format        REW - MPS with generic names\n")
		fmt.Printf("LP   - CPLEX LP format   ALP - LP with generic names\n")
		fmt.Printf("\nEnter file type: ")
		fmt.Scanln(&userString)
		fileType = strings.ToUpper(userString)
			
		switch fileType {
				
		case "QUIT":
			return
					
		case "SAV", "MPS", "REW", "LP", "ALP":
			fileName = ""
			fmt.Printf("Enter file name: ")
			fmt.Scanln(&fileName)
			if custEnvOn {
				fileName = dSrcDev + fileName + fExtension
			}

			if err = gpx.WriteProb(fileName, fileType); err != nil {
				fmt.Printf("Failed with: %s\n", err)
			} else {
				fmt.Printf("Saving file '%s', type '%s', was successful.\n", 
							fileName, fileType)
			}	

		default:
			fmt.Printf("Unsupported file type: %s\n", fileType)
							
		} // end switch on file type
	} // end while processing model files		
	
}

//==============================================================================

// wpPrintGpxIn prints the gpx input data structures. The function accepts no
// arguments and returns no values.
func wpPrintGpxIn() {
	var userString string  // user input
	var counter    int     // counter keeping track of number of lines printed

	if gName != "" {
		fmt.Printf("\nProblem name: %s\n", gName)		
	} else {
		fmt.Printf("WARNING: Problem name is empty.\n")
	}

	if len(gObj) != 0 {
		fmt.Printf("\nDisplay the objective function list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
	
			fmt.Printf("\nObjective Function List:\n")
			fmt.Printf("%5s %8s %15s", "i", "Col#", "Value\n")
			counter = 0
			for i := 0; i < len(gObj); i++ {
				fmt.Printf("%5d %8d %15e\n", i, gObj[i].ColIndex, gObj[i].Value)
				counter++
				userString = ""
				if counter == pauseAfter {
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}			
				} // end if pause needed
			} // end for obj list
		} // end if displaying list
	} else {
		fmt.Printf("WARNING: Objective function list is empty.\n")
	}

	if len(gRows) != 0 {
		fmt.Printf("\nDisplay rows list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			fmt.Printf("\nRows List:\n")
			fmt.Printf("%5s %5s %15s   %15s %15s\n", "i", "Sense", "Name", "RHS", "Range")
			counter = 0
			for i := 0; i < len(gRows); i++ {
				fmt.Printf("%5d %5s %15s   %15e %15e\n", i, gRows[i].Sense, gRows[i].Name, 
				 		gRows[i].Rhs, gRows[i].RngVal)
				counter++
				userString = ""
				if counter == pauseAfter {
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}			
				} // end if pause needed
			} // end for rows list
		} // end if displaying list		
	} else {
		fmt.Printf("WARNING: Rows list is empty.\n")
	}	

	if len(gCols) != 0 {
		fmt.Printf("\nDisplay columns list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			fmt.Printf("\nColumns List:\n")
			fmt.Printf("%5s %5s %15s   %15s %15s\n", "i", "Type", "Name", "Lower Bound", "Upper Bound")
			counter = 0
			for i := 0; i < len(gCols); i++ {
				fmt.Printf("%5d %5s %15s   %15e %15e\n", i, gCols[i].Type, gCols[i].Name, 
					 	gCols[i].BndLo, gCols[i].BndUp)
				counter++
				userString = ""
				if counter == pauseAfter {
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}			
				} // end if pause needed
			} // end for rows list
		} // end if displaying list		
	} else {
		fmt.Printf("WARNING: Columns list is empty.\n")
	}	

	if len(gElem) != 0 {
		fmt.Printf("\nDisplay elements list [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			fmt.Printf("\nNon-zero Elements List:\n")
			fmt.Printf("%5s %5s %5s  %15s\n", "i", "inRow", "inCol", "Value")
			counter = 0
			for i := 0; i < len(gElem); i++ {
				fmt.Printf("%5d %5d %5d   %15e\n", i, gElem[i].RowIndex, gElem[i].ColIndex, 
					 	gElem[i].Value)
				counter++
				userString = ""
				if counter == pauseAfter {
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}			
				} // end if pause needed
			} // end for rows list
		} // end if displaying list
	} else {
		fmt.Printf("WARNING: Elements list is empty.\n")
	}	
		
}

//==============================================================================

// wpPrintGpxSoln prints the gpx solution data structures. It accepts no arguments
// and returns no values.
func wpPrintGpxSoln() {
	var userString string  // user input
	var counter    int     // counter keeping track of number of lines printed
	
	fmt.Printf("\nObjective function value = %f\n\n", sObjVal)
	
	userString = ""
	fmt.Printf("Display additional results [Y|N]: ")
	fmt.Scanln(&userString)

	if userString == "y" || userString == "Y" {
		if len(sRows) != 0 {
			counter = 0
			for i := 0; i < len(sRows); i++ {
				fmt.Printf("Row %4d: %15s, Pi = %13e,  Slack = %13e\n", 
							i, sRows[i].Name, sRows[i].Pi, sRows[i].Slack)
				counter++
				userString = ""
				if counter == pauseAfter {
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}			
				} // end if pause needed
			} // end for printing constraints

			fmt.Printf("\nPAUSED... hit any key to continue: ")
			fmt.Scanln(&userString)
		
		} else {
			fmt.Printf("List of solved constraints is empty.\n")
		}
		

		if len(sCols) != 0 {
			counter = 0
			for i := 0; i < len(sCols); i++ {
				fmt.Printf("Col %4d: %15s, Val = %13e,  Reduced cost = %13e\n", 
							i, sCols[i].Name, sCols[i].Value, sCols[i].RedCost)
				counter++
				userString = ""
				if counter == pauseAfter {
					fmt.Printf("\nPAUSED... <CR> continue, any key to quit: ")
					fmt.Scanln(&userString)
					if userString != "" {
						break 
					}		
				} // end if pause needed
			} // end for printing variables
						
		} else {
			fmt.Printf("List of solved variables is empty.\n")
		}			

	} // end if printing results
		
}

//==============================================================================

// runGpxWrapper executes functions from the GPX package. 
// The display of menu items may be hidden to avoid clutter, but the command
// options remain available even if the menu item is hidden. 
// The function is called from the main wrapper and accepts the cmdOption as an 
// argument. If the command cannot be executed because it does not match any of 
// the cases covered by this wrapper, it returns an error.
func runGpxWrapper(cmdOption string) error {	
	var userString    string        // holder for strings input by user
	var tmpString     string        // temp holder for string variables
	var tmpInt        int           // temp holder for int variables
	var tmpBool       bool          // temp holder for boolean variable
	var err           error         // error returned by functions called

	// The gpx variables used in this function are package global variables so
	// we don't have to pass them to the higher-level wrapper and back again as
	// individual commands that use them are executed.
	
	switch cmdOption {

	//--------------------------------------------------------------------------
	case "61":
		fmt.Printf("Creating element list.\n")
		if err = gpx.ChgCoefList(gElem); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Non-zero elements have been set.\n")				
		}			

	//--------------------------------------------------------------------------
	case "62":
		// ChgObjSense
		fmt.Printf("Specify problem type [max|min]: ")
		fmt.Scanf(userString)
		tmpString = strings.ToUpper(userString)
		switch tmpString {
			
		case "MAX":
			err = gpx.ChgObjSen(-1)
			
		case "MIN":
			err = gpx.ChgObjSen(1)
			
		default:
			fmt.Printf("Unsupported problem type: %s\n", userString)
			break
		} // end switch on problem type
		
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Problem sense set to '%s'\n", tmpString)
		}
			
	//--------------------------------------------------------------------------
	case "63":
		fmt.Printf("Enter new problem name: ")
		fmt.Scanf(userString)
		if err = gpx.ChgProbName(userString); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Problem name changed to '%s'.\n", userString)
		}
			
	//--------------------------------------------------------------------------
	case "64":
		fmt.Printf("Closing Cplex.\n")
		if err = gpx.CloseCplex(); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Cplex closed successfully.\n", userString)
		}
	
	//--------------------------------------------------------------------------
		case "65":
		fmt.Printf("Enter name for new problem: ")
		fmt.Scanln(&userString)
		if err = gpx.CreateProb(userString); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("New problem with name '%s' created.\n", userString)
		}

	//--------------------------------------------------------------------------
	case "66":
		// GetColName - create a new list and populate with column names
		tmpInt = 0
		if err = gpx.GetNumCols(&tmpInt); err != nil {
			// Cannot get number of rows.
			fmt.Println(err)
		} else {
			sCols = nil
			sCols = make([]gpx.SolnCol, tmpInt)
			if err = gpx.GetColName(sCols)  ; err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("New solution list populated with column names.\n")
			} // end else retrieved column names				
		} // end else retrieved number of columns correctly

	//--------------------------------------------------------------------------
	case "67":
		fmt.Printf("Obtaining MIP solution.\n")
		err = gpx.GetMipSolution(&sObjVal, &sRows, &sCols)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("MIP solution obtained successfully.\n")
		}
			
	//--------------------------------------------------------------------------
	case "68":
		tmpInt = 0
		err = gpx.GetNumCols(&tmpInt)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Problem has %d columns.\n", tmpInt)
		}
			
	//--------------------------------------------------------------------------
	case "69":
		tmpInt = 0
		err = gpx.GetNumRows(&tmpInt)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Problem has %d rows.\n", tmpInt)
		}

	//--------------------------------------------------------------------------
	case "70":
		sObjVal = 0.0
		err = gpx.GetObjVal(&sObjVal)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Objective function value = %f\n", sObjVal)
		}
		
	//--------------------------------------------------------------------------
	case "71":
		// GetRowName - create a new list and populate with row names
		tmpInt = 0
		if err = gpx.GetNumRows(&tmpInt); err != nil {
			// Cannot get number of rows.
			fmt.Println(err)
		} else {
			sRows = nil
			sRows = make([]gpx.SolnRow, tmpInt)
			if err = gpx.GetRowName(sRows) ; err != nil {
				fmt.Println(err)
			} else {
				fmt.Printf("New solution list populated with row names.\n")
			} // end else added slack values				
		} // end else retrieved number of rows correctly

	//--------------------------------------------------------------------------
	case "72":
		// GetSlack - populate existing list with retrieved slack values
		tmpInt = 0
		if err = gpx.GetNumRows(&tmpInt); err != nil {
			// Cannot get number of rows.
			fmt.Println(err)
		} else {
			if tmpInt != len(sRows) {
				// Got number of rows, but it does not match size of our list.
				fmt.Printf("Cplex row list size is %d, available list size is %d.\n",
						tmpInt, len(sRows))
				fmt.Printf("Use the 'GetRowName' option to create list of correct size.\n")
			} else {
				// Have right-size list, try to populate it.
				if err = gpx.GetSlack(sRows); err != nil {
					fmt.Println(err)
				} else {
					fmt.Printf("Slack values added to existing solution row list.\n")
				} // end else added slack values				
			} // end else list sizes match
		} // end else retrieved number of rows correctly
		
	//--------------------------------------------------------------------------
	case "73":
		err = gpx.GetSolution(&sObjVal, &sRows, &sCols)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("LP solution obtained successfully.\n")
		}
	
	//--------------------------------------------------------------------------	
	case "74":
		// GetX - populate existing list with retrieved variable values
		tmpInt = 0
		if err = gpx.GetNumCols(&tmpInt); err != nil {
			// Cannot get number of columns.
			fmt.Println(err)
		} else {
			if tmpInt != len(sCols) {
				// Got number of cols, but it does not match size of our list.
				fmt.Printf("Cplex column list size is %d, available list size is %d.\n",
						tmpInt, len(sCols))
				fmt.Printf("Use the 'GetColName' option to create list of correct size.\n")
			} else {
				// Have right-size list, try to populate it.
				if err = gpx.GetX(sCols); err != nil {
					fmt.Println(err)
				} else {
					fmt.Printf("X values added to existing solution column list.\n")
				} // end else added X values				
			} // end else list sizes match
		} // end else retrieved number of cols correctly

	//--------------------------------------------------------------------------
	case "75":
		fmt.Printf("Optimizing existing LP.\n")
		if err = gpx.LpOpt(); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("LP optimized successfully by Cplex.\n")
		}	
	
	//--------------------------------------------------------------------------
	case "76":
		fmt.Printf("Optimizing existing MIP.\n")
		if err = gpx.MipOpt(); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("MIP optimized successfully by Cplex.\n")
		}	

	//--------------------------------------------------------------------------
	case "77":
		fmt.Printf("Creating new cols.\n")
		if err = gpx.NewCols(gObj, gCols); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("New columns created in Cplex.\n")
		}

	//--------------------------------------------------------------------------
	case "78":
		fmt.Printf("Creating new rows.\n")
		if err = gpx.NewRows(gRows); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("New rows created in Cplex.\n")
		}

	//--------------------------------------------------------------------------
	case "79":
		fmt.Printf("Display output to screen [Y|N]: ")
		fmt.Scanln(&userString)
		if userString == "y" || userString == "Y" {
			tmpBool = true	
		} else {
			tmpBool = false
		}
		
		if err = gpx.OutputToScreen(true); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Cplex output to screen set to '%t'.\n", tmpBool)
		}

	//--------------------------------------------------------------------------
	case "80":
		if err = wpReadDataFile(); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("File defining the model was read successfully.\n")
		}

	//--------------------------------------------------------------------------
	case "81":
		userString = ""
		fmt.Printf("Enter file name for writing solution: ")
		fmt.Scanln(&userString)
		if custEnvOn {
			userString = dSrcDev + userString + fExtension
		}

		if err = gpx.SolWrite(userString); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Solution written to file '%s'.\n", userString)
		}

	//--------------------------------------------------------------------------
	case "82":
		wpWriteProb()


	default:
		return errors.Errorf("Command %s not in functions menu", cmdOption)
		
	} // End switch on command option

	
	return nil	
}

