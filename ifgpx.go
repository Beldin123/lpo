// // +build exclude

//==============================================================================
// ifgpx: Interface Functions for GPX
// 01   July  29, 2018   Functions from other lpo files moved here
// 02   Aug.  28, 2018   Modified lporun, added support for Coin-OR


// Any function which makes use of the gpx package is in this file.
// This makes the other lpo files independent of gpx. If gpx is not installed,
// this file must be excluded from the build to avoid compilation errors.
//
// The primary function is CplexSolveProb, which in turn calls other functions 
// to read, reduce, and solve the model and process the solution.

package lpo

import (
	"github.com/pkg/errors"
	"github.com/go-opt/gpx"
)

//==============================================================================
// GENERAL UTILITY PRIVATE FUNCTIONS
//==============================================================================

// buildCpxVarMap returns the map of variables built from the cplex output. 
// In case of failure, it returns an error.
func buildCpxVarMap(scaleMap map[string]float64, cpSoln []gpx.SolnCol, varbMap *PsResVarMap) error {

	newMap := make(PsResVarMap)	
		
	for i := 0; i < len(cpSoln); i++ {		

		scaleFactor, ok := scaleMap[cpSoln[i].Name];
		if !ok {
			return errors.Errorf("Missing scale factor for variable %s", cpSoln[i].Name)			
		}
		
		mapItem := newMap[cpSoln[i].Name]
		mapItem.Value       = cpSoln[i].Value
		mapItem.ScaleFactor = scaleFactor
		mapItem.ReducedCost = cpSoln[i].RedCost
		mapItem.Status      = psVarStatNA
					
		newMap[cpSoln[i].Name] = mapItem
	} // End for processing varbs list

	*varbMap = newMap
		
	return nil	
}

//==============================================================================

// buildCpxConMap returns the map of constraints built from the cplex output.
// In case of failure, function returns an error.
func buildCpxConMap(cpSoln []gpx.SolnRow, constrMap *PsResConMap) error {
	
	newMap := make(PsResConMap)	
	
	for i := 0; i < len(cpSoln); i++ {
		
		mapItem := newMap[cpSoln[i].Name]

		// Transfer values received from cplex		
		mapItem.Status = psVarStatNA
		mapItem.Slack  = cpSoln[i].Slack
		mapItem.Pi     = cpSoln[i].Pi
		// TODO: The dual value is not automatically provided by CPX functions
		// as it is in the xml file, and would need to be obtained separately.
		mapItem.Dual   = 0

		// Initialize all other values that we may fill in later
		mapItem.Rhs    = 0
		mapItem.Type   = "X"
		
		newMap[cpSoln[i].Name] = mapItem
	}

	*constrMap = newMap	

	return nil
}

//==============================================================================
// EXPORTED FUNCTIONS
//==============================================================================

// CplexSolveProb receives a control structure specifying the MPS input file to be read,
// the file where Cplex output should be written (default will be used if not
// specified), the maximum number of iterations lpo should perform, and boolean
// flags indicating which reduction operations to perform and whether to solve
// the problem.
//
// The function then reads the MPS input file and reduces the problem size by
// iteratively performing the reduction operations specified. If the RunSolver 
// flag is set to false, the function returns at this point.
//
// If the RunSolver flag is set to true, the function then passes the reduced 
// model to Cplex to be solved either as an LP, or as a MIP. It
// then processes the results provided by Cplex and, in conjunction with data
// stored in internal structures about the presolve operations, reconstitues the
// original problem, and returns the result in the psRslt data structure. 
//
// Shadow price and slack values which are typically calculated by Cplex are 
// not calculated for variables and constraints that have been removed during 
// presolving, and hence not passed to Cplex. Those values are set to 0, and the
// status associated with that value is set to "NA" (not available).
//
// In case of failure, function returns an error.
func CplexSolveProb (psc PsCtrl, psRslt *PsSoln) error {
	var numRows            int  // number of rows in the model prior to reduction
	var numCols            int  // number of cols in the model prior to reduction
	var numElem            int  // number of elements in the model prior to reduction
	var coefPerLine        int  // number of coef./line to be printed by WritePsopFile
	var objVal         float64  // value of the objective function returned by Cplex
	var sRows    []gpx.SolnRow  // solved constraints returned by Cplex via gpx
	var sCols    []gpx.SolnCol  // solved variables returned by Cplex via gpx
	var conMap     PsResConMap  // constraint map merged from Cplex and reduction results
	var varMap     PsResVarMap  // variable map  merged from Cplex and reduction results
	var origObjFunc      psRow  // objective function before reductions in post-solve format
	var psRows         []psRow  // original constraints translated to post-solve format
	var err              error  // error returned by secondary functions called
	var colScaleMap  map[string]float64  // map of column scale factors in original model
	

	// Initialize variables.
	psOpList       = nil
	psRslt.ObjVal  = 0
	psRslt.ConMap  = nil
	psRslt.VarMap  = nil
	psRslt.RowsDel = 0
	psRslt.ColsDel = 0
	psRslt.ElemDel = 0
	coefPerLine    = 2

	if psc.FileInMps != "" {
		if err = ReadMpsFile(psc.FileInMps); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed to read file")
		}
		
		// Check that none of the other files have the same name so we don't
		// accidentally overwrite our input file.
		
		if psc.FileInMps == psc.FileOutSoln {
			return errors.Errorf("Cplex solution file cannot overwrite %s", psc.FileInMps)
		}
		
		if psc.FileInMps == psc.FileOutMpsRdcd {
			return errors.Errorf("MPS output file cannot overwrite %s", psc.FileInMps)
		}

		if psc.FileInMps == psc.FileOutPsop {
			return errors.Errorf("PSOP output file cannot overwrite %s", psc.FileInMps)
		}
		
	} // End if populating model from file

	// Record original matrix size.
	numRows = len(Rows)
	numCols = len(Cols)
	numElem = len(Elems)

	// If lists of rows, cols, or elems is empty, return an error.
	
	if numRows <= 0 {
		return errors.Errorf("CplexSolveProb received empty rows list")	
	}
	if numCols <= 0 {
		return errors.Errorf("CplexSolveProb received empty columns list")	
	}
	if numElem <= 0 {
		return errors.Errorf("CplexSolveProb received empty elements list")	
	}

		
	// Translate all original rows to the new format and save the objective function,
	// if it exists, as a separate entity also in the new format.
	
	_ = translateAllRows(&psRows)
	
	if ObjRow >= 0 {
		// If objective function is not the first row in the list, move it there.
		if ObjRow != 0 {
			log(pINFO, "\nMoving %s from index %d to top of list.\n", Rows[ObjRow].Name, ObjRow)
			_ = swapRows(0, ObjRow)
			ObjRow = 0	
		}

		// There is an objective function, save it for later use.
		if err = translateRow(Rows[ObjRow], &origObjFunc); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed")
		}
	}	

	// Make the map which of column scale factors and populate map until the values
	// can be transferred to the Cplex solution.

	colScaleMap = make(map[string]float64)
	
	for i := 0; i < len(Cols); i++ {
		colScaleMap[Cols[i].Name] = Cols[i].ScaleFactor	
	}

	// Remove rows and columns specified in the control structure and calculate
	// how many rows, cols, and elems were removed.			
	if err = ReduceMatrix(psc); err != nil {
		return errors.Wrap(err, "CplexSolveProb failed")
	}
	
	psRslt.RowsDel = numRows - len(Rows)
	psRslt.ColsDel = numCols - len(Cols)
	psRslt.ElemDel = numElem - len(Elems)


	// Write the reduced MPS file if requested.	
	if psc.FileOutMpsRdcd != "" {
		if err = WriteMpsFile(psc.FileOutMpsRdcd); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed")		
		}
	}

	// Write the Psop file if requested.
	if psc.FileOutPsop != "" {
		if err = WritePsopFile(psc.FileOutPsop, coefPerLine); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed")		
		}				
	}

	// Solution was not requested, so return at this point
	if ! psc.RunSolver {
		return nil		
	}

	// Create the LP using callable C functions.
	if err = CplexCreateProb(); err != nil {
		return errors.Wrap(err, "CplexSolveProb failed")
	}	

	if isMip() {
		// This is a MIP, so use the CPX functions for mixed integer problems.
		if err = gpx.MipOpt(); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed to optimize MIP")				
		}

		if err = gpx.GetMipSolution(&objVal, &sRows, &sCols); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed to get solution")				
		}
				
	} else {
		// This is an LP, so use the CPX functions for LP.
		if err = gpx.LpOpt(); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed to optimize LP")				
		}

		if err = gpx.GetSolution(&objVal, &sRows, &sCols); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed to get solution")				
		}
		
	} // End else this is LP


	// Build the variable and constraint maps, transfer data from original model
	// and merge with results obtained from Cplex.	
	if err = buildCpxVarMap(colScaleMap, sCols, &varMap); err != nil {
		return errors.Wrap(err, "CplexSolveProb failed to process variables")				
	}
	
	_ = buildCpxConMap(sRows, &conMap)

	psRslt.ConMap = conMap
	psRslt.VarMap = varMap

	// Write the Cplex solution to xml file if requested.
	if psc.FileOutSoln != "" {
		if err = gpx.SolWrite(psc.FileOutSoln); err != nil {
			return errors.Wrap(err, "CplexSolveProb failed to write solution to file")		
		}		
	}

	// Close and clean up Cplex.
	if err = gpx.CloseCplex(); err != nil {
		return errors.Wrap(err, "CplexSolveProb failed to close cplex")
	}
	
	// Update the maps with the information deleted during presolve.
	if err = postSolve(psRslt.ConMap, psRslt.VarMap); err != nil {
		return errors.Wrap(err, "CplexSolveProb failed")
	}

	// Restore all other rows that may have been removed and not put back during
	// pre- and postsolve (e.g. non-binding).

	for i := 0; i < len(psRows); i++ {
		_ = addConMapItem(psRslt.ConMap, psRows[i])
	}

	for i := 0; i < len(psRows); i++ {
				
		if mapItem, ok := psRslt.ConMap[psRows[i].Name]; ok {

			mapItem.Dual        = psRslt.ConMap[psRows[i].Name].Dual
			mapItem.Rhs         = psRslt.ConMap[psRows[i].Name].Rhs
			mapItem.Slack       = psRslt.ConMap[psRows[i].Name].Slack 
			mapItem.Status      = psRslt.ConMap[psRows[i].Name].Status
			mapItem.Type        = psRslt.ConMap[psRows[i].Name].Type
			mapItem.ScaleFactor = psRslt.ConMap[psRows[i].Name].ScaleFactor
			psRslt.ConMap[psRows[i].Name] = mapItem
		} else {
			log(pERR, "ERROR: Row %s not found in map.\n", psRows[i].Name)
		}
	}
	 	
	// Calculate the proper value of the objective function.
    // There may have been a constant associated with the objective function 
	// which must now be included in the calculation.
	
	if err = getPstLhs(origObjFunc, psRslt.VarMap, &psRslt.ObjVal); err != nil {
		return errors.Wrap(err, "CplexSolveProb failed")		
	}

	psRslt.ObjVal -= objRowConst
		
	return nil
}

//==============================================================================

// TransToGpx translates lpo data structures to the gpx data structures which will
// in turn be translated to internal C data structures passed to Cplex. The input
// is taken from the Rows, Cols, and Elems global variables, and the output is
// returned via the variables defined below.
// In case of failure, function returns an error.
//
//	The arguments used by this function are:
//	   gRows [output]: rows, except the objective function, in gpx format
//	   gCols [output]: columns in gpx format
//	   gElem [output]: non-zero elements present in the gRows list
//	   gObj  [output]: non-zero elements present in the objective function
func TransToGpx(gRows *[]gpx.InputRow, gCols *[]gpx.InputCol, gElem *[]gpx.InputElem, 
				gObj *[]gpx.InputObjCoef) error {

	var rowItem   gpx.InputRow     // row item used in constructing gRows list
	var colItem   gpx.InputCol     // col item used in constructing gCols list
	var elemItem  gpx.InputElem    // elem item used in constructing gElem list
	var objItem   gpx.InputObjCoef // item used in constructing coefficients in obj. func.
	var rowIndex  int              // index or row being processed
	var elemIndex int              // index of element being processed

	// Initialize lists that will be returned.
	*gRows = nil
	*gCols = nil
	*gElem = nil
	*gObj  = nil

	// Problem needs to have some rows.
	if len(Rows) == 0 {
		return errors.Errorf("Input list of rows is empty")				
	}

	// Problem needs to have some columns.
	if len(Cols) == 0 {
		return errors.Errorf("Input list of columns is empty")				
	}

	// Problem needs to have some elements.
	if len(Elems) == 0 {
		return errors.Errorf("Input list of elements is empty")				
	}

	// The objective function must be one of the rows.
	if ObjRow < 0 || ObjRow >= len(Rows) {
		return errors.Errorf("Unexpected location of objective function row %d", ObjRow)		
	}

	// Translate the columns data structure
	for i := 0; i < len(Cols); i++ {
		colItem.Name  = Cols[i].Name
		colItem.BndLo = Cols[i].BndLo
		colItem.BndUp = Cols[i].BndUp

		switch Cols[i].Type {

			// At this time, lpo only differentiates between Real and Integer
			// variables. Map them to the values Cplex understands and flag anything
			// else.
			case "R":
				colItem.Type  = "C"
				
			case "I":
				colItem.Type  = "I"

			default:
				return errors.Errorf("Unexpected type %s in col %s", Cols[i].Type, Cols[i].Name)			
			
		}
		
		*gCols = append(*gCols, colItem)		
	}
	
	// Translate the rows data structure. The objective row is handled as a
	// separate entity, because that's how Cplex expects it. We also need to adjust
	// indices to account for one of the rows being the objective function.
	rowIndex = 0
	for i := 0; i < len(Rows); i++ {

		if i == ObjRow {
			// Translate the objective function to its data structure.
			for j := 0; j < len(Rows[i].HasElems); j++ {
				elemIndex = Rows[i].HasElems[j]
				objItem.ColIndex = Elems[elemIndex].InCol
				objItem.Value    = Elems[elemIndex].Value
				*gObj = append(*gObj, objItem)
			} // End for objective function coefficients
		} else {

			rowItem.Name   = Rows[i].Name
			rowItem.Sense  = Rows[i].Type
			rowItem.RngVal = 0.0
			
			switch Rows[i].Type {
				case "L", "E":	
					rowItem.Rhs   = Rows[i].RHSup
				
				case "G":
					rowItem.Rhs   = Rows[i].RHSlo

				case "R":
					rowItem.Rhs   =  Rows[i].RHSlo
					rowItem.RngVal = Rows[i].RHSup - Rows[i].RHSlo

				case "N":
					// In case "N" row type gets past all other safeguards,
					// translate to something that Cplex will not reject.				
					if Rows[i].RHSup != Plinfy {
						rowItem.Sense = "L"
						rowItem.Rhs   = Rows[i].RHSup
					} else if Rows[i].RHSlo != -Plinfy {
						rowItem.Sense = "G"
						rowItem.Rhs   =  Rows[i].RHSlo
					} else {
						return errors.Errorf("Failed to translate non-binding row %s", Rows[i].Name)						
					}
					
							
				default:
					return errors.Errorf("Unexpected type %s in row %s", Rows[i].Type, Rows[i].Name)			
			} // End switch on row type	

			*gRows = append(*gRows, rowItem)
			
			// Translate the non-zero elements. We need to use "rowIndex" to compensate
			// for the first row in the LPO structures being the Objective Function.
			// The column index will match the LPO structures and can be used "as is".
			
			for j := 0; j < len(Rows[i].HasElems); j++ {
				elemIndex           = Rows[i].HasElems[j]
				elemItem.RowIndex   = rowIndex
				elemItem.ColIndex   = Elems[elemIndex].InCol
				elemItem.Value      = Elems[elemIndex].Value
				*gElem = append(*gElem, elemItem)
			}
			
			rowIndex++
								
		} // End else processing rows other than objective function					
	} // End for all rows

	return nil	
}

//==============================================================================

// TransFromGpx translates gpx data structures to the global variables
// Rows, Cols, and Elems in order to create the model.
// In case of failure, function returns an error.
//
//	The arguments used by this function are:
//	   probNm [input]: name of the problem
//	   objNm  [input]: name of the objective function
//	   gRows  [input]: rows, except the objective function, in gpx format
//	   gCols  [input]: columns in gpx format
//	   gElem  [input]: non-zero elements present in the gRows list
//	   gObj   [input]: non-zero elements present in the objective function
func TransFromGpx(probNm string, objNm string, gRows []gpx.InputRow, gCols []gpx.InputCol, 
					gElem []gpx.InputElem, gObj []gpx.InputObjCoef) error {

	var rowItem  InputRow   // temporary holder for row item
	var colItem  InputCol   // temporary holder for column item
	var elemItem InputElem  // temporary holder for element item
	var err      error      // error returned by called functions

	// Check that the received rows, columns, and elements lists are not empty.
	// The other input parameters can be empty and are not checked.
	
	// Problem needs to have some rows.
	if len(gRows) == 0 {
		return errors.Errorf("List of gpx rows is empty")				
	}

	// Problem needs to have some columns.
	if len(gCols) == 0 {
		return errors.Errorf("List of gpx columns is empty")				
	}

	// Problem needs to have some elements.
	if len(gElem) == 0 {
		return errors.Errorf("List of gpx elements is empty")				
	}

	// Initialize the lpo data structures and set the model name.
	
	if err = InitModel(); err != nil {
		return errors.Wrap(err, "Failed to initialize model")
	}	

	// Transfer the problem name passed to function to global variable.
	Name = probNm

	// Translate the columns data structure, and keep same order as in original.
	for i := 0; i < len(gCols); i++ {
		colItem.Name  = gCols[i].Name
		colItem.BndLo = gCols[i].BndLo
		colItem.BndUp = gCols[i].BndUp
		
		switch gCols[i].Type {

			case "C": // Continuous aka real variable
				colItem.Type  = "R"
				
			case "I", "B": // General integer or binary variable
				colItem.Type  = "I"

			case "S": // Semi-continuous variable
				log(pWARN, "WARNING: Only the continuous part of a semi-continuous variable is handled. Lower bound = 1.0.\n")
				colItem.Type  = "I"

			case "N": // TODO: Check if warning needed for semi-integer variable
				colItem.Type  = "I"

			default:
				return errors.Errorf("Unexpected type %s in col %s", Cols[i].Type, Cols[i].Name)			
			
		} // End switch on column type
		
		Cols = append(Cols, colItem)		
	} // End if processing columns


	// Translate the rows data structure, and keep same order as in original.
	for i := 0; i < len(gRows); i++ {
		
		rowItem.Name   = gRows[i].Name
		rowItem.Type   = gRows[i].Sense

		switch gRows[i].Sense {
			case "L":	
				rowItem.RHSlo = -Plinfy
				rowItem.RHSup = gRows[i].Rhs

			case "E":	
				rowItem.RHSlo = gRows[i].Rhs
				rowItem.RHSup = gRows[i].Rhs
				
			case "G":
				rowItem.RHSlo = gRows[i].Rhs
				rowItem.RHSup = Plinfy

			case "R":
				rowItem.RHSlo = gRows[i].Rhs
				rowItem.RHSup = gRows[i].Rhs + gRows[i].RngVal
											
			default:
				return errors.Errorf("Unexpected type %s in row %s", Rows[i].Type, Rows[i].Name)			
			} // End switch on row type	

		Rows = append(Rows, rowItem)
				
	} // End if processing rows
	
	
	// Translate the non-zero elements structure, and keep same order as in original.
	for i := 0; i < len(gElem); i++ {

		elemItem.Value = gElem[i].Value
		elemItem.InRow = gElem[i].RowIndex
		elemItem.InCol = gElem[i].ColIndex

		// Update the Rows and Cols lists to point at the current element
				
		Elems = append(Elems, elemItem)
		Rows[elemItem.InRow].HasElems = append(Rows[elemItem.InRow].HasElems, i)
		Cols[elemItem.InCol].HasElems = append(Cols[elemItem.InCol].HasElems, i)		

	} // End for all non-zero elements
	
	// Process the objective function if it is present, and add it to the end of
	// the rows list.

	if len(gObj) > 0 {
		// If objective function name not received, set it to default.
		if objNm == "" {
			objNm = "ObjFunc"
		}

		rowItem.Name  = objNm
		rowItem.Type  = "N"
		rowItem.RHSlo = 0.0
		rowItem.RHSup = 0.0

		// Build up the elements list for the objective function. Be careful about
		// order of adding items to Elems and Rows lists since the lengths
		// of these lists are used as array indices in the HasElems list of obj. func.
		for i := 0; i < len(gObj); i++ {
			elemItem.Value   = gObj[i].Value
			elemItem.InRow   = len(Rows)
			elemItem.InCol   = gObj[i].ColIndex
			rowItem.HasElems = append(rowItem.HasElems, len(Elems))
			Elems            = append(Elems, elemItem)	
		}

		// Finally add the objective function to the end of the rows list.
		Rows = append(Rows, rowItem)
				
	} // End if objective function is present
			
	
	// Move objective function to top of list and calculate gradient vectors.
	
	if err = AdjustModel(); err != nil {
		return errors.Wrap(err, "Failed to adjust model")		
	}
	
	return nil									
}

//==============================================================================

// CplexCreateProb initializes the Cplex environment, translates the model from
// the global Rows, Cols, and Elems variables to data structures used by the gpx
// package, and uses gpx to build the model in Cplex so that it may be solved by
// separate function calls.
// In case of failure, function returns an error.
func CplexCreateProb() error {
	var gRows []gpx.InputRow     // rows data structure, excluding the objective function
	var gCols []gpx.InputCol     // cols data structure
	var gElem []gpx.InputElem    // non-zero elements present in the rows structure
	var gObj  []gpx.InputObjCoef // non-zero elements in the objective function
	var err     error            // error returned by secondary functions called

	// Initialize the Cplex environment and assign the problem name based on the
	// lpo global problem "Name".	
	if err = gpx.CreateProb(Name); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create problem")
	}

	// If logLevel is high enough, have output printed to screen.
	if logLevel >= pINFO {
		if err = gpx.OutputToScreen(true); err != nil {
			return errors.Wrap(err, "CplexCreateProb ailed to set output to screen")
		}
	}

	// Translate from lpo global data structures to the gpx data structures.	
	err = TransToGpx(&gRows, &gCols, &gElem, &gObj)
	if err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to translate to gpx data structures")
	}

	// Populate the rows of the problem.
	if err = gpx.NewRows(gRows); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create rows")		
	}			

	// Populate the columns and objective function of the problem.
	if err = gpx.NewCols(gObj, gCols); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create columns")		
	}			

	// Change the coefficients of the problem to their non-zero values.
	if err = gpx.ChgCoefList(gElem); err != nil {
		return errors.Wrap(err, "CplexCreateProb failed to create elements")		
	}			

	return nil			
}

//============================ END OF FILE =====================================