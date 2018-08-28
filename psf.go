//==============================================================================
// psf: PreSolve Functions
// 01   July  5, 2018   Initial version uploaded to github
// 02   Aug. 28, 2018   Modified lporun, added support for Coin-OR


// This file contains auxiliary functions for LPO which reduce the original problem 
// set by deleting rows and columns as per Andersen and Andersen paper (1993).
// This file also contains functions which make use of Cplex, but are independent
// of the gpx package (i.e. interaction is via files rather than callable routines).

package lpo

import (
	"encoding/xml"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)


// PsCtrl specifies which components should be invoked, and how many iterations
// should be performed by the various components when reducing the size of the
// LP model. It is passed as an argument to CplexSolveProb or CoinSolveProb.
type PsCtrl struct {
	FileInMps         string  // MPS input file, or "" for none
	FileOutSoln       string  // Solver solution (xml) output file, or "" for none
	FileOutMpsRdcd    string  // Reduced MPS output file, or "" for none
	FileOutPsop       string  // Output file of pre-solve operations, or "" for none
	MaxIter           int     // Maximum iterations for lpo
	DelRowNonbinding  bool    // Controls if non-binding rows are removed
	DelRowSingleton   bool    // Controls if row singletons are removed
	DelColSingleton   bool    // Controls if column singletons are removed
	DelFixedVars      bool    // Controls if fixed variables are removed
	RunSolver         bool    // Controls if problem is to be solved by the solver 		
}

// PsSoln returns the results from CplexSolveProb or CoinSolveProb to the caller. 
// It contains the value of the objective function, the row and column maps of the 
// LP, and the numbers of rows, columns, and elements that were removed during 
// presolve operations.
type PsSoln struct{
	ObjVal  float64       // Value of the objective function
	ConMap  PsResConMap   // Map of string to structs for constraints
	VarMap  PsResVarMap   // Map of string to structs for variables   
	RowsDel int           // Number of rows removed during presolve
	ColsDel int	          // Number of columns removed during presolve
	ElemDel int           // Number of elements removed during presolve	
}

// PsResConMap contains the map of constraints included in PsSoln that is
// returned to the caller. It contains values calculated by the solver where available,
// or Status = NA and zero values if constaint was removed during presolve and not
// processed by the solver.
type PsResConMap map[string] struct {
	Status      string    // Reserved for future use, always set to "NA"
	Type        string    // Inequality type provided as model input
	Rhs         float64   // RHS provided as model input
	ScaleFactor float64   // Row scale factor used in original model
	Pi          float64   // Dual solution (Pi) from solver, 0 if not available
	Slack       float64   // Slack from solver, 0 if not available
	Dual        float64   // Dual from solver, 0 if not available
}

// PsResVarMap contains the map of variables included in PsSoln that is returned to
// the caller. It contains values calculated by the solver where available,
// or Status = NA and zero values if constaint was removed during presolve and not
// processed by the solver.
type PsResVarMap map[string] struct {
	Status      string    // Reserved for future use, always set to "NA"		
	Value       float64   // Variable value from the solver or postsolve calculation
	ScaleFactor float64   // Variable scale factor used in original model
	ReducedCost float64   // Reduced cost from the solver if available, 0 if postsolved
}

// psOp is used internally to record the presolve operation performed
// and the row and/or column that was removed during that operation.
type psOp struct {
	OpType  string  // Type of reduction operation performed 
	Col     psCol   // Column deleted from model by this operation (may be nil)
	Row     psRow   // Row deleted from model by this operation (may be nil)
}

// psRow is used internally in the list of presolve operations (psOp) to store
// data about the row that was removed.
type psRow struct {
	Name        string   // Row name
	Type        string   // Row type
	Rhs         float64  // Row RHS
	ScaleFactor float64  // Row scale factor
	Coef       []psCoef  // List of coefficients and variables for this row	
}

// psCoef is part of the psRow structure to store the list of variable names and
// their coefficients for a particular row (psRow) processed by presolve operations (psOp).
type psCoef struct {
	Name   string   // Variable name
	Value  float64  // Coefficient value
}

// psCol is used internally in the list of presolve operations (psOp) to store
// data about the column that was removed.
type psCol struct {
	Name        string   // Column name
	Type        string   // Column type
	BndLo       float64  // Lower bound
	BndUp       float64  // Upper bound
	ScaleFactor float64  // Column scale factor
}


// Constants used to determine which presolve operation was performed
const (
	psopFreeCol      = "FCS"   // Free and implied free column singleton
	psopFixedVar     = "FXV"   // Fixed variable
	psopRowSingltn   = "RSG"   // Row Singleton
	psopNbRow        = "NBR"   // Non-binding row
	psopEmptyCol     = "MTC"   // Empty column
	psopEmptyRow     = "MTR"   // Empty row
)

// Delimiter for sections in PSOP file
const fileDelim = "#------------------------------------------------------------------------------\n"

const (
	psVarStatNA   = "NA"    // Var state provided by the solver not available
	psConStatNA   = "NA"    // Costr state provided by the solver not available
)

// Package global variables
var psOpList []psOp                     // Rows and cols deleted during presolve
var defaultCplexInput =  "cplexIn.txt"  // MPS file storing reduced matrix
var defaultCplexOutput = "cplexOut.txt" // File storing cplex solution


//==============================================================================
// GENERAL UTILITY AND POST-SOLVE FUNCTIONS
//==============================================================================

// getPstLhs returns the LHS of a constraint calculated from the constraint and 
// variables map passed to the function.
// In case of failure, it returns an error.
func getPstLhs(psRow psRow, psVarMap PsResVarMap, lhs *float64) error {

	*lhs = 0.0
		
	for i := 0; i < len(psRow.Coef); i++ {
		
		if psVar, ok := psVarMap[psRow.Coef[i].Name]; !ok {
			return errors.Errorf("Failed to find LHS for %s", psRow.Coef[i].Name) 
		} else {			
			*lhs += psVar.Value * psRow.Coef[i].Value		
		} 
	}

	// Adjust the calculated value by the scale factor.
	*lhs = *lhs * psRow.ScaleFactor
	
	return nil
}

//==============================================================================

// isMip checks if the problem is considered a MIP according to the solver. It scans
// the columns, and if it detects any type other than "R" (which is translated
// to "continuous" for Cplex, it is considered a MIP, and the function returns
// "true". If it does not find one, it returns false and the problem can be solved
// as a pure linear problem (LP).
func isMip() bool {
	
	for i := 0; i < len(Cols); i++ {
		if Cols[i].Type != "R" {
			return true
		}
	}
	
	return false
}

//==============================================================================

// translateRow translates a single constraint (oldRow) to the psRow format 
// and returns it as newRow in the argument list.
// In case of failure, function returns an error.
func translateRow(oldRow InputRow, newRow *psRow) error {
	var iCurElem        int  // index of item in Elems list being processed
	var coef         psCoef  // coefficient item being processed
	var psCoefList []psCoef  // list of coefficients associated with this row

	newRow.Name = ""
	newRow.Type = ""
	newRow.Coef = nil
	newRow.Rhs = 0
	
	newRow.Name        = oldRow.Name
	newRow.Type        = oldRow.Type
	newRow.ScaleFactor = oldRow.ScaleFactor

	switch oldRow.Type {
	case "G", "E", "N":
		newRow.Rhs = oldRow.RHSlo

	case "L":
		newRow.Rhs = oldRow.RHSup

	} // End switch on row type
			

	for i := 0; i < len(oldRow.HasElems); i++ {

		iCurElem   = oldRow.HasElems[i]
		coef.Name  = Cols[Elems[iCurElem].InCol].Name
		coef.Value = Elems[iCurElem].Value
		psCoefList = append(psCoefList, coef)
	} // End for all coefficients in this row

	newRow.Coef = psCoefList

	return nil	
}

//==============================================================================

// translateAllRows returns the list of all rows (newRowList) in the psRow format 
// constructed from the original Rows, Cols, and Elems global variables. The new
// row format is needed to eliminate any cross references via indices to other
// lists, because the indices will change as rows and cols are deleted.
// In case of failure, function returns an error.
func translateAllRows(newRowList *[]psRow) error {
	var newRow psRow  // new row item being processed
	
	*newRowList = nil
	
	for i := 0; i < len(Rows); i++ {

		_      = translateRow(Rows[i], &newRow)
		*newRowList = append(*newRowList, newRow)	
	}
	
	return nil	
}

//==============================================================================

// addConMapItem updates the constraint map referenced by the psConMap argument
// by adding information from argument referenced by psRow. It is used by the
// interface to Cplex as well as the one to Coin-OR when processing the solution.
// In case of failure, it returns an error.
func addConMapItem(psConMap PsResConMap, psRow psRow) error {

	if mapItem, ok := psConMap[psRow.Name]; !ok {
		// Item is new.
		// Initialize values which would be returned from cplex.
		mapItem.Status      = psConStatNA
		mapItem.Slack       = 0
		mapItem.Dual        = 0

		// Transfer values passed in
		mapItem.Rhs         = psRow.Rhs
		mapItem.Type        = psRow.Type
		mapItem.ScaleFactor = psRow.ScaleFactor
		
		psConMap[psRow.Name] = mapItem		
	} else {
		// Item exists, but we need to restore original Rhs and type which were
		// modified as a result of presolve.
		// Initialize values which would be returned from cplex.	
		mapItem.Status = psConMap[psRow.Name].Status
		mapItem.Slack  = psConMap[psRow.Name].Slack
		mapItem.Dual   = psConMap[psRow.Name].Dual

		// Transfer values passed in
		mapItem.Rhs         = psRow.Rhs
		mapItem.Type        = psRow.Type
		mapItem.ScaleFactor = psRow.ScaleFactor
			
		psConMap[psRow.Name] = mapItem			
	}

	return nil
		
}

//==============================================================================

// updatePsList adds an item to the package global list of presolve operations
// (psOpList) which is constructed from the operation type (opType), as well as
// the row and/or column specified by rowIndex and colIndex, respectively,
// that are removed during the operation. A single row, a single column, or a row
// as well as a column may be removed.
// In case of failure, function returns an error.
func updatePsList(opType string, rowIndex int, colIndex int) error {
	var psItem psOp   // item associated with a post-solve operation list
	var err   error   // error returned by called functions
	
	// Transfer all variables "as is" from Cols data structures, if a column
	// was deleted.

	psItem.OpType          = opType	
	
	if colIndex >= 0 && colIndex < len(Cols) {
		psItem.Col.Name        = Cols[colIndex].Name
		psItem.Col.Type        = Cols[colIndex].Type
		psItem.Col.BndLo       = Cols[colIndex].BndLo
		psItem.Col.BndUp       = Cols[colIndex].BndUp
		psItem.Col.ScaleFactor = Cols[colIndex].ScaleFactor		
	} // End if a column was deleted	
	
	// If a row was deleted, translate it to the new format and add it to
	// the item that will be added to the list.

	if rowIndex >= 0 && rowIndex < len(Rows) {
		if err = translateRow(Rows[rowIndex], &psItem.Row); err != nil {
			return errors.Wrapf(err, "updatePsList failed with row %d", rowIndex)
		}
		
	} // End if row was deleted

	// Add the new item to the existing global list.	
	psOpList = append(psOpList, psItem)

	return nil
}


//==============================================================================

// postSolve updates the constraint map (pscMap) and variable map (solvedVarMap)
// referenced in the argument list with information stored in the package global
// list of presolve operations (psOpList). Effectively, this function merges the
// results calculated by the solver with results obtained by "reversing" the presolve
// operations. In case of failure, function returns an error.
func postSolve(pscMap PsResConMap, solvedVarMap PsResVarMap) error {
	var rhs   float64  // RHS of row being processed
	var lhs   float64  // LHS of row being processed
	var coef  float64  // holder for value of coefficient being processed
	var curVar psCoef  // holder for variable structure being processed
		
	for i := len(psOpList) - 1; i >= 0; i-- {
	
		switch psOpList[i].OpType {

		// Operations recorded so they could be printed, but which don't need
		// any post-solve steps and can be ignored
		case psopEmptyRow, psopNbRow:
			continue
						
		// Fixed Variable ------------------------------------------------------	
		case psopFixedVar:

			// Calculate variable value and add it to solved variables map.
			// There is no row information to transfer for this item.
			varbMap := make(PsResVarMap)
			vMapItem := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = psOpList[i].Col.BndLo
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem

		// Empty Column ------------------------------------------------------	
		case psopEmptyCol:

			// Set the value to zero, since at removal it was in range from 0
			// to infinity in either positive or negative direction.
			varbMap := make(PsResVarMap)
			vMapItem := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = 0
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem
				
		// Free Column Singleton -----------------------------------------------	
		case psopFreeCol:	

			// First get the RHS and coefficient value
			rhs = psOpList[i].Row.Rhs
			lhs = 0
			coef = 0
			for j := 0; j < len(psOpList[i].Row.Coef); j++ {
				curVar.Name = psOpList[i].Row.Coef[j].Name
				if curVar.Name == psOpList[i].Col.Name {
					// If this is the varb we are looking for, get coef. for this row
					coef = psOpList[i].Row.Coef[j].Value					
				} else {
					// If different varb, sum up its value times coef (lhs) for this row
					if psVar, ok := solvedVarMap[curVar.Name]; !ok {
						return errors.Errorf("postSolve unable to find value for %s", curVar.Name)
					} else {
						lhs += psVar.Value * psOpList[i].Row.Coef[j].Value
					} // End else increment lhs value
				} // End else not variable we need to solve
			} // End for all variables in row
			
			// Calculate variable value and add it to solved variables map
			varbMap             := make(PsResVarMap)
			vMapItem            := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = (rhs - lhs) / coef
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem

			// Get deleted row details and add it to solved constraints map
			constrMap      := make(PsResConMap)
			cMapItem       := constrMap[psOpList[i].Row.Name]
			cMapItem.Type        = psOpList[i].Row.Type
			cMapItem.Rhs         = psOpList[i].Row.Rhs
			cMapItem.Dual        = 0
			cMapItem.Slack       = 0
			cMapItem.Status      = psConStatNA
			cMapItem.ScaleFactor = psOpList[i].Row.ScaleFactor
			pscMap[psOpList[i].Row.Name] = cMapItem
			

		// Row Singleton -------------------------------------------------------	
		case psopRowSingltn:

			rhs = psOpList[i].Row.Rhs
			coef = 0
			for j := 0; j < len(psOpList[i].Row.Coef); j++ {
				if psOpList[i].Row.Coef[j].Name == psOpList[i].Col.Name {
					coef = psOpList[i].Row.Coef[j].Value
					break
				} 
			}
			
			if coef == 0 {
				return errors.New("postSolve unable to find coefficient")
			}
			
			// Calculate variable value and add it to solved variables map
			varbMap             := make(PsResVarMap)
			vMapItem            := varbMap[psOpList[i].Col.Name]
			vMapItem.Value       = rhs / coef
			vMapItem.ReducedCost = 0
			vMapItem.Status      = psVarStatNA
			vMapItem.ScaleFactor = psOpList[i].Col.ScaleFactor
			solvedVarMap[psOpList[i].Col.Name] = vMapItem

			// Get deleted row details and add it to solved constraints map
			constrMap      := make(PsResConMap)
			cMapItem       := constrMap[psOpList[i].Row.Name]
			cMapItem.Type        = psOpList[i].Row.Type
			cMapItem.Rhs         = psOpList[i].Row.Rhs
			cMapItem.Dual        = 0
			cMapItem.Slack       = 0
			cMapItem.Status      = psConStatNA
			cMapItem.ScaleFactor = psOpList[i].Row.ScaleFactor
			pscMap[psOpList[i].Row.Name] = cMapItem
					

		// Something unknown ---------------------------------------------------	
		default:
			return errors.Errorf("Unexpected operation %s in postSolve", psOpList[i].OpType)
		} // End switch on operation type
		
	} // End for processing psOpList
	
	return nil
}

//==============================================================================
// ROW REDUCTION OPERATIONS
//==============================================================================

// swapRows switches the rows specified by source and destination indices 
// (srcIndex, destIndex) in Rows list and updates all cross-references.
// In case of failure, it returns an error.
func swapRows(srcIndex int, destIndex int) error {
	var tempRow InputRow  // holder for row being moved
	var index        int  // holder for index of item being processed

	// Return error if indices are out of range

	if srcIndex < 0 || srcIndex >= len(Rows) {
		return errors.Errorf("Source index %d out of range in swapRows", srcIndex)
	}

	if destIndex < 0 || destIndex >= len(Rows) {
		return errors.Errorf("Dest. index %d out of range in swapRows", destIndex)
	}

	// Swap the references to the two rows in the elements lists.

	for i := 0; i < len(Rows[srcIndex].HasElems); i++ {
		index = Rows[srcIndex].HasElems[i]
		Elems[index].InRow = destIndex
	}

	for i := 0; i < len(Rows[destIndex].HasElems); i++ {
		index = Rows[destIndex].HasElems[i]
		Elems[index].InRow = srcIndex
	}

	// Swap the two rows.

	tempRow         = Rows[destIndex]
	Rows[destIndex] = Rows[srcIndex]
	Rows[srcIndex]  = tempRow

	return nil
}


//==============================================================================

// DelRow deletes the row specified by index srcRow, and updates all cross references.
// If srcRow is not already the last row in the list, the row is swapped with the
// last row and is deleted from the end of the list.
// In case of failure, it returns an error.
func DelRow(srcRow int) error {
	var iCurElem       int   // index of current element
	var iLastElem      int   // index of last element in list
	var lastRow        int   // index of last row in list
	var index          int   // holder for index being processed
	var elemList     []int   // list of element associated with item
	var newElemList  []int   // new element list created after items deleted
	var tempElem InputElem   // temporary holder for element
	var err          error   // error received from called functions

	lastRow = len(Rows) - 1
	
	// Check that index of row to be deleted is valid.
	if srcRow < 0 || srcRow > lastRow {
		return errors.Errorf("Row index %d out of range", srcRow)
	}

	// If row to be deleted is not the last row, swap rows to put it at end of list.
	if srcRow != lastRow {
		if err = swapRows(srcRow, lastRow); err != nil {
			return errors.Wrap(err, "Row swap failed")
		}			
	}

	// Step through the list of elements in the row to be deleted and migrate
	// them to the end of the elements list by swapping with those that remain.
	// Use temporary elemList to keep track of all elements that need to be
	// processed because the HasElems list associated with the current row may be changing.
	
	iLastElem = len(Elems) - 1
	elemList  = Rows[lastRow].HasElems
	
	for i := 0; i < len(elemList); i++ {

		iCurElem = elemList[i]

		// Remove element to be deleted from the column where it occurs.
		index = Elems[iCurElem].InCol
		newElemList = nil
		for j := 0; j < len(Cols[index].HasElems); j++ {
			if Cols[index].HasElems[j] != iCurElem {
				newElemList = append(newElemList, Cols[index].HasElems[j])
			}
		}
		Cols[index].HasElems = newElemList
		
		// Find	the row location of the former last element and update reference.
		index = Elems[iLastElem].InRow
		for j := 0; j < len(Rows[index].HasElems); j++ {
			if Rows[index].HasElems[j] == iLastElem {
				Rows[index].HasElems[j] = iCurElem
				break
			}
		}

		// Find the column location of the former last element and update reference.
		index = Elems[iLastElem].InCol
		for j := 0; j < len(Cols[index].HasElems); j++ {
			if Cols[index].HasElems[j] == iLastElem {
				Cols[index].HasElems[j] = iCurElem
				break
			}
		}
		
		// Swap the elements and update index of next available slot.
		tempElem         = Elems[iLastElem]
		Elems[iLastElem] = Elems[iCurElem]
		Elems[iCurElem]  = tempElem
		iLastElem--	
		
	} // End for all elements of row being deleted.

	// Reslice the rows and elements lists.
	Elems = append(Elems[:iLastElem + 1])
	Rows  = append(Rows[:len(Rows) - 1])
	
	return nil
}

//==============================================================================

// DelCol deletes the column specified by srcCol and updates all cross references.
// If the column to be deleted is not already the last column in the list, it is
// swapped with the last column and is deleted from the end of the list.
// In case of failure, it returns an error.
func DelCol(srcCol int) error {
	var err          error  // error string returned from other functions
	var lastCol        int  // index of last column in the global list
	var iCurElem       int  // index of current element being processed
	var iLastElem      int  // index of last element in global list
	var index          int  // general variable for storing indices as needed
	var elemList     []int  // list of elements being processed
	var newElemList  []int  // new list excluding elements that were deleted
	var tempElem InputElem  // placeholder for swapping items in element list


	lastCol = len(Cols) - 1

	// Exit if index we received is out of range.
	if srcCol < 0 || srcCol > lastCol {
		return errors.Errorf("Column index %d out of range", srcCol)
	}

	// If column to be deleted is not already the last one, swap with last column.
	if srcCol != lastCol {
		if err = swapCols(srcCol, lastCol); err != nil {
			return errors.Wrap(err, "Column swap failed")			
		}	
	}
	
	// Step through the list of elements in the column to be deleted and migrate
	// them to the end of the elements list by swapping with those that remain.
	// Use temporary elemList to keep track of all elements that need to be
	// processed because the Elem list associated with the current column may be changing.
	
	iLastElem = len(Elems) - 1
	elemList  = Cols[lastCol].HasElems

	for i := 0; i < len(elemList); i++ {
	
		iCurElem = elemList[i]
				
		// Remove element to be deleted from row where it occurs.
		index       = Elems[iCurElem].InRow
		newElemList = nil
		for j := 0; j < len(Rows[index].HasElems); j++ {
			if Rows[index].HasElems[j] != iCurElem {
				newElemList = append(newElemList, Rows[index].HasElems[j])
			}	
		}
		Rows[index].HasElems = newElemList

		// Find	the row location of the former last element and update reference.
		index = Elems[iLastElem].InRow
		for j := 0; j < len(Rows[index].HasElems); j++ {
			if Rows[index].HasElems[j] == iLastElem {
				Rows[index].HasElems[j] = iCurElem
				break
			}
		}

		// Find the column location of the former last element and update reference.
		index = Elems[iLastElem].InCol
		for j := 0; j < len(Cols[index].HasElems); j++ {
			if Cols[index].HasElems[j] == iLastElem {
				Cols[index].HasElems[j] = iCurElem
				break
			}
		}
		
		// Swap the elements and update index of next available slot.
		tempElem          = Elems[iLastElem]
		Elems[iLastElem] = Elems[iCurElem]
		Elems[iCurElem]  = tempElem

		iLastElem--				
	}

	// Reslice the rows and elements lists.
	Elems = append(Elems[:iLastElem + 1])
	Cols  = append(Cols[:len(Cols) - 1])
	
	return nil
}

//==============================================================================

// delTaggedRows finds rows tagged for deletion, moves them to end of list by 
// swapping with still-active rows, deletes all tagged rows, and updates 
// cross-references. Function passes back the number of rows deleted in the numDltd
// variable.
// In case of failure, function returns an error.
func delTaggedRows(numDltd *int) error {
	var err error    // error received from called functions

	// Delete any rows tagged for deletion by looking from the end of the list.
	// Swapping of rows to put them at the end is done by the deletion function.
	for i := len(Rows) - 1; i >= 0; i-- {

		if Rows[i].State == stateDelete {

			if err = DelRow(i); err != nil {
				return errors.Wrapf(err, "Failed to delete row %d", i)
			} // End if row deletion failed

			*numDltd = *numDltd + 1

		} // End if found row to delete
	} // End for all rows in list

	return nil
}

//==============================================================================

// delNbRows searches the Rows list for any non-binding rows that are still
// in the active state and deletes them. It passes back the number of rows deleted
// in the numDltd variable.
// In case of failure, function returns an error.
func delNbRows(numDltd *int) error {
	var err error  // error received from called functions
		
	log(pINFO, "Looking for non-binding rows...\n")
	
	*numDltd = 0

	// Skip over rows if they are not active.	
	for i := 0; i < len(Rows); i++ {

		if Rows[i].State != stateActive {
			continue
		}
		
		if Rows[i].Type == "N" {
			Rows[i].State = stateDelete	
			_ = updatePsList(psopNbRow, i, -1)
		}	
	} // End for looping over all rows	

	if err = delTaggedRows(numDltd); err != nil {
		return errors.Wrap(err, "delNbRows failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d non-binding rows.\n", *numDltd)
	}
	
	return nil
}

//==============================================================================

// delEmptyRows searches the Rows list for any empty rows that are still
// in the active state and deletes them. It passes back the number of rows deleted
// in the numDltd variable.
// In case of failure, function returns an error.
func delEmptyRows(numDltd *int) error {
	var err error  // error received from called functions
		
	log(pINFO, "Looking for empty rows...\n")

	*numDltd = 0
	
	for i := 0; i < len(Rows); i++ {

		// Skip over any rows that are not still active or that are not empty	
		if  len(Rows[i].HasElems) > 0 || Rows[i].State != stateActive {
			continue
		}

		// Lower bounds may not be correct.	
		if Rows[i].RHSlo == -Plinfy && Rows[i].RHSup != 0 {
			log(pWARN, "WARNING: Empty row %s has bounds %f to %f.\n",
				Rows[i].Name, Rows[i].RHSlo, Rows[i].RHSup)
		}	

		// Upper bounds may not be correct.	
		if Rows[i].RHSlo != 0 && Rows[i].RHSup == Plinfy {
			log(pWARN, "WARNING: Empty row %s has bounds %f to %f.\n",
				Rows[i].Name, Rows[i].RHSlo, Rows[i].RHSup)
		}	

		Rows[i].State = stateDelete
		_ = updatePsList(psopEmptyRow, i, -1)
		log(pDEB, "  Row %s removed.\n", Rows[i].Name)
	
	} // End for all rows

	if err = delTaggedRows(numDltd); err != nil {
		return errors.Wrap(err, "delEmptyRows failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d empty rows.\n", *numDltd)		
	}

	return nil
}

//==============================================================================

// delEmptyCols searches the Cols list for any empty columns that are still
// in the active state and deletes them. It passes back the number of columns deleted
// in the numDltd variable.
// In case of failure, function returns an error.
func delEmptyCols(numDltd *int) error {
	var err error  // error received from called functions
		
	log(pINFO, "Looking for empty columns...\n")

	*numDltd = 0
	
	for i := 0; i < len(Cols); i++ {

		// Skip over any cols that are not still active or that are not empty	
		if  len(Cols[i].HasElems) > 0 || Cols[i].State != stateActive {
			continue
		}

		// Lower bounds may not be correct.	
		if Cols[i].BndLo == -Plinfy && Cols[i].BndUp != 0 {
			log(pWARN, "WARNING: Empty col %s has bounds %f to %f.\n",
				Cols[i].Name, Cols[i].BndLo, Cols[i].BndUp)
		}	

		// Upper bounds may not be correct.	
		if Cols[i].BndLo != 0 && Cols[i].BndUp != Plinfy {
			log(pWARN, "WARNING: Empty col %s has bounds %f to %f.\n",
				Cols[i].Name, Cols[i].BndLo, Cols[i].BndUp)
		}	

		Cols[i].State = stateDelete
		_ = updatePsList(psopEmptyCol, -1, i)
		log(pDEB, "  Col %s removed.\n", Cols[i].Name)
	
	} // End for all rows

	if err = delTaggedCols(numDltd); err != nil {
		return errors.Wrap(err, "delEmptyCols failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d empty columns.\n", *numDltd)		
	}

	return nil	
}

//==============================================================================
// COLUMN REDUCTION OPERATIONS
//==============================================================================

// swapCols switches columns specified by source and destination indices 
// (srcIndex, destIndex) in Cols list and updates all cross-references.
// In case of failure, it returns an error.
func swapCols(srcIndex int, destIndex int) error {
	var tempCol InputCol  // temporary holder for column as we swap them
	var index        int  // temporary holder for index needed during processing

	// Return error if indices are out of range.

	if srcIndex < 0 || srcIndex >= len(Cols) {
		return errors.Errorf("Source index %d out of range in swapCols", srcIndex)
	}

	if destIndex < 0 || destIndex >= len(Cols) {
		return errors.Errorf("Destination index %d out of range in swapCols", destIndex)
	}

	// Swap references to the two columns in the elements lists.

	for i := 0; i < len(Cols[srcIndex].HasElems); i++ {
		index = Cols[srcIndex].HasElems[i]
		Elems[index].InCol = destIndex
	}

	for i := 0; i < len(Cols[destIndex].HasElems); i++ {
		index = Cols[destIndex].HasElems[i]
		Elems[index].InCol = srcIndex
	}

	// Swap the columns.

	tempCol         = Cols[destIndex]
	Cols[destIndex] = Cols[srcIndex]
	Cols[srcIndex]  = tempCol

	return nil
}

//==============================================================================

// delFixedVars deletes all fixed variables and passes back the number of columns 
// deleted in the numDltd variable.
// In case of failure, function returns an error.
func delFixedVars(numDltd *int) error {
	var index    int  // holder for index of list currently being processed
	var coef float64  // coefficient value
	var err    error  // error returned by secondary functions

	// Initialize variables
	*numDltd = 0

	log(pINFO, "Looking for fixed variables ...\n")
	
	// Look through the columns list for items that have same upper & lower bound
	for i := 0; i < len(Cols); i++ {

		// Only process active rows, and skip over locked or deleted ones.
		if Cols[i].State != stateActive {
			continue
		}
		
		if Cols[i].BndLo != Cols[i].BndUp {
			// Not a fixed variable, move to the next one.
			continue
		}

		// Tag the column for deletion and add it to the list of cols deleted.
		log(pDEB, "  Col %s removed.\n", Cols[i].Name)
		Cols[i].State = stateDelete				
		_ = updatePsList(psopFixedVar, -1, i)

		// Update RHS for each row where this column appears.
		for j := 0; j < len(Cols[i].HasElems); j++ {

			index = Elems[Cols[i].HasElems[j]].InRow
			coef  = Elems[Cols[i].HasElems[j]].Value

			if Rows[index].RHSlo != -Plinfy {
				Rows[index].RHSlo -= Cols[i].BndLo * coef
			}

			if Rows[index].RHSup != Plinfy {
				Rows[index].RHSup -= Cols[i].BndUp * coef
			}
			
		} // End for all rows associated with fixed variable
				
	} // End for all columns


	if err = delTaggedCols(numDltd); err != nil {
		return errors.Wrap(err, "delFixedVars failed")
	}
	
	if *numDltd != 0 {
		log(pINFO, "Deleted %d fixed variables.\n", *numDltd)		
	}
	
	return nil
}

//==============================================================================

// delTaggedCols finds columns tagged for deletion, moves them to end of list by 
// swapping with still-active columns, deletes all tagged columns, and updates 
// cross-references. Function passes back the number of columns deleted in the
// numDltd variable.
// In case of failure, function returns an error.
func delTaggedCols(numDltd *int) error {
	var err error  // error value received from called functions

	// Initialize variables
	*numDltd = 0

	// Delete all tagged columns, working from the end of the list. Swapping of
	// columns to put them at the end is done by the deletion function.

	for i := len(Cols) - 1; i >= 0; i-- {

		if Cols[i].State == stateDelete {
			if err = DelCol(i); err != nil {
				return errors.Wrapf(err, "Failed to delete column %d", i)				
			}

		*numDltd = *numDltd + 1
		
		} // End if column tagged for deletion
	} // End for deleting all tagged columns

	return nil
}

//==============================================================================

// delFreeColSingls deletes free column singletons, defined as a variable 
// which appears only in the objective function and has bounds from negative
// infinity to positive infinity. It passes the number of items (rows and cols) 
// deleted back in the numDltd variable.
// In case of failure, function returns an error.
func delFreeColSingls(numDltd *int) error {
	var rowIndex  int  // holder for index of list item currently being processed
	var rowsFound int  // number of rows found and deleted
	var colsFound int  // number of columns found and deleted
	var err     error  // error received from called functions

	*numDltd = 0
	rowIndex = -1
	
	for i := 0; i < len(Cols); i++ {

		if len(Cols[i].HasElems) != 1 {
			// Variable occurs in more than one place, can't be removed.
			continue
		}
		
		if Cols[i].BndLo != -Plinfy || Cols[i].BndUp != Plinfy {
			// Not a free variable, can't be removed.
			continue
		} 
		
		rowIndex =  Elems[Cols[i].HasElems[0]].InRow
		if rowIndex == ObjRow {
			// Variable occurs only in objective function, can't be removed.
			log(pDEB, "Variable %s in objective %s, not in any constraint.\n",
				Cols[i].Name, Rows[ObjRow].Name)
			continue
		}
		
		// Tag the column and row for deletion, and add them to postsolve list.
		
		log(pINFO, "  Row %s and col %s removed.\n", Rows[rowIndex].Name, Cols[i].Name)
		
		Cols[i].State = stateDelete
		Rows[rowIndex].State = stateDelete
		_ = updatePsList(psopFreeCol, rowIndex, i)
						
	} // End for all columns	

	// Delete the rows and columns, if row deletion fails return at that point,
	// otherwise delete columns and return with the appropriate return code and
	// number of items deleted.
	
	if err = delTaggedRows(&rowsFound); err != nil {
		*numDltd = rowsFound
		return errors.Wrap(err, "delFreeColSingls row deletion failed")	
	}
	
	err = delTaggedCols(&colsFound)
	*numDltd = rowsFound + colsFound
	if err != nil {
		return errors.Wrap(err, "delFreeColSingls col deletion failed")
	}
	
	if rowsFound != 0 || colsFound != 0 {
		log(pINFO, "Deleted %d rows and %d cols.\n", rowsFound, colsFound)
	}

	return nil	
}

//==============================================================================

// delRowSingletons searches the Rows list for any singleton rows that are still
// in the active state and deletes them. It passes the number of rows deleted back
// in the numDltd variable.
// In case of failure, function an error.
func delRowSingletons(numDltd *int) error {
	var colIndex     int  // column index of item being processed
	var rowIndex     int  // row index of item being processed
	var colsFound    int  // number of columns found and deleted
	var rowsFound    int  // number of rows found and deleted
	var coef     float64  // coefficient value
	var newBound float64  // updated RHS value for row being processed
	var err        error  // error received from secondary functions

	// Initialize variables
	*numDltd  = 0
	rowsFound = 0
	colsFound = 0

	log(pINFO, "Looking for row singletons...\n")
	
	// Find all rows that contain a single variable.

	for i := 0; i < len(Rows); i++ {

		// Only process active rows, skip over locked and deleted ones.		
		if Rows[i].State != stateActive {
			continue
		}
		
		if len(Rows[i].HasElems) == 1 {


			if Rows[i].Type != "E" {
				// Skip any inequalities.
				continue
			}
			
			colIndex = Elems[Rows[i].HasElems[0]].InCol
			coef     = Elems[Rows[i].HasElems[0]].Value
			//log(pTRC, "Found singleton row [%d-%s], col [%d-%s]\n",
			//	i, Rows[i].Name, colIndex, Cols[colIndex].Name)

			// Don't want any divisions by zero, so check just in case.
			if coef == 0 {
				log(pERR, "Error: Unexpected zero coef for Row %s, Col %s.\n",
					Rows[i].Name, Cols[colIndex].Name)
				return nil
			}

			// Set the variable bounds depending on the row type.
			switch Rows[i].Type {

			// TODO: Andersen algorithm may only work for equality constraints, and
			// the other cases can be removed. For now, leave them in until confirmed.
			case "G":
				newBound = Rows[i].RHSlo / coef
				Cols[colIndex].BndLo = newBound

			case "L":
				newBound = Rows[i].RHSup / coef
				Cols[colIndex].BndUp = newBound

			case "E", "N":
				newBound = Rows[i].RHSlo / coef
				Cols[colIndex].BndLo = newBound
				Cols[colIndex].BndUp = newBound

			} // End switch on row type

			// Adjust the RHS of each constraint where this variable occurs
			
			for j := 0; j < len(Cols[colIndex].HasElems); j++ {
				rowIndex =  Elems[Cols[colIndex].HasElems[j]].InRow
				coef     =  Elems[Cols[colIndex].HasElems[j]].Value
				//log(pTRC, "  j = %d, row %s\n", j, Rows[rowIndex].Name)
				
				if i == rowIndex {
					//log(pDEB, "Skipping target row [%d] %s\n", i, Rows[i].Name)
					continue
				}
				
				if Rows[rowIndex].RHSlo != -Plinfy {
					Rows[rowIndex].RHSlo -= newBound * coef
				}
				
				if Rows[rowIndex].RHSup != Plinfy {
					Rows[rowIndex].RHSup -= newBound	* coef
				}				
			} // End for all rows where this variable occurs
			

			// Delete row and column and update list and counters

			Rows[i].State        = stateDelete
			Cols[colIndex].State = stateDelete
			_ = updatePsList(psopRowSingltn, i, colIndex)
			log(pINFO, "  Row %s and col %s removed.\n", Rows[i].Name, Cols[colIndex].Name)
			//log(pTRC, "    Row lo %f up %f, var lo %f up %f\n", Rows[i].RHSlo, Rows[i].RHSup,
			//	Cols[colIndex].BndLo, Cols[colIndex].BndUp)
									
		} // End if we found singleton row
	} // End for all rows in the list

	// Delete the rows and columns, if row deletion fails return at that point,
	// otherwise delete columns and return with the appropriate return code and
	// number of items deleted.
	
	if err = delTaggedRows(&rowsFound); err != nil {
		*numDltd = rowsFound
		return errors.Wrap(err, "delRowSingeltons failed")	
	}

	err = delTaggedCols(&colsFound)
	*numDltd = rowsFound + colsFound
	if err != nil {
		return errors.Wrap(err, "delRowSingletons failed")
	}
	
	if rowsFound != 0 || colsFound != 0 {
		log(pINFO, "Deleted %d rows and %d cols.\n", rowsFound, colsFound)
	}

	return nil
}

//==============================================================================
// EXPORTED FUNCTIONS
//==============================================================================

// ReduceMatrix iteratively performs the reduction operations specified in the psControl 
// structure to remove rows and columns from the model until no more reductions
// occur, or until the maximum number of iterations is reached. The function also 
// performs some additional reductions (e.g. removal of empty rows) which are not configurable.
//
// In case of failure, the function returns an error.
//
//	The fields of the psControl structure have the following meaning for this function:
//	   MaxIter           int    - maximum iterations for reduction loop
//	   DelRowNonbinding  bool   - if true, remove non-binding rows
//	   DelRowSingleton   bool   - if true, remove row singletons
//	   DelColSingleton   bool   - if true, remove column singletons
//	   DelFixedVars      bool   - if true, remove fixed variables
//	   RunSolver         bool   - ignored by this function 		
//	   FileInMps         string - ignored by this function
//	   FileOutSoln       string - ignored by this function
//	   FileOutMpsRdcd    string - ignored by this function
//	   FileOutPsop       string - ignored by this function
func ReduceMatrix(psControl PsCtrl) error {
	var itemsFound  int  // number of items deleted by a specific operation
	var itemsInPass int  // number of changes made in current iteration
	var numChanges  int  // number of changes made in all iterations
	var totalIter   int  // number of iterations performed by TightenBounds
	var err       error  // error returned by secondary functions called

	numChanges = 0
	
	for i := 1; i <= psControl.MaxIter; i++ {

		// Iterate over row and column reductions until no more changes in the
		// number of elements are observed. The NumElements global counter is 
		// updated whenever rows or columns are deleted.
		
		itemsInPass = 0
		
		log(pINFO, "\nIteration %d: %d rows, %d cols, %d elements.\n", i,
			len(Rows), len(Cols), len(Elems))

		if psControl.DelRowNonbinding {

			if err = TightenBounds(psControl.MaxIter, &totalIter); err != nil {
				return errors.Wrap(err, "TightenBounds failed")		
			}
			
			if err = delNbRows(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")				
			}
					
			itemsInPass += itemsFound
		} // End if non-binding row


		if psControl.DelFixedVars || psControl.DelRowNonbinding {
			// This component must be executed if non-binding rows were removed.
			if err = delFixedVars(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")
			}

			itemsInPass += itemsFound
		} // End if fixed variable


		if psControl.DelRowSingleton {
			if err = delRowSingletons(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")							
			}
			
			itemsInPass += itemsFound			
		} // End if row singleton	

						
		if psControl.DelColSingleton {
			if err = delFreeColSingls(&itemsFound); err != nil {
				numChanges += itemsFound
				return errors.Wrap(err, "ReduceMatrix failed")								
			}
			
			itemsInPass += itemsFound						
		} // End if column singleton

		// Empty rows are deleted automatically without any configurable flag.	
		if err = delEmptyRows(&itemsFound); err != nil {
			numChanges += itemsFound
			return errors.Wrap(err, "ReduceMatrix failed")											
		}

		// Empty cols are deleted automatically without any configurable flag.	
		if err = delEmptyCols(&itemsFound); err != nil {
			numChanges += itemsFound
			return errors.Wrap(err, "ReduceMatrix failed")											
		}

		// Increment counters and print status when done.
		itemsInPass += itemsFound		
		numChanges  += itemsInPass
				
		if itemsInPass == 0 {
			log(pINFO, "Reduction done after %d of %d iterations, %d items removed.\n", 
					i, psControl.MaxIter, numChanges)
			break
		}

	} // End for maximum iterations loop
	
	return nil
}

//==============================================================================

// WritePsopFile writes the rows and columns that were removed during the pre-solve
// operations to a text file specified by the user. The function accepts two
// arguments, fileName and coefPerLine. If the file name the file to which the
// output is written, and if empty, a default name is used. 
//
// The coefPerLine specifies how many coefficient name/value pairs should be 
// written per line and is interpretted as follows:
//	  < 0 - all pairs are written on a single line (no CR/LF is inserted between pairs)
//	    0 - printing of coefficient name/value pairs is suppressed
//	    n - a carriage return line feed is inserted after printing n pairs  
// In case of failure, the function returns an error.
func WritePsopFile(fileName string, coefPerLine int) error {

	var opName       string // operation name in more detail than internal var. 
	var rowPresent   bool   // flag indicating that row needs to be printed
	var colPresent   bool   // flag indicating that column needs to be printed
	var index        int    // index tracking how many coefficients were printed
	var printCoef    bool   // controls if coef name/value pairs are printed
	var coefCrNeeded bool   // controls if <CR> printed between coef name/value pairs


	//Check whether the file exists. If it exists, overwrite it.

	if _, err := os.Stat(fileName); err == nil {
		err = os.Remove(fileName)
		if err != nil {
			return errors.Wrapf(err, "Failed to delete existing file %s", fileName)
		}
	}
	
	f, err := os.Create(fileName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create new file %s", fileName)
	}

 	defer f.Close()

	// Set flags to control printing of coefficient name/value pairs per line
	printCoef    = true
	coefCrNeeded = true
	
	if coefPerLine < 0	{
		coefCrNeeded = false
		coefPerLine *= -1
	} else if coefPerLine == 0 {
		printCoef = false
	}

	// Print status message to screen and general header into the file.
	log(pINFO, "\nWriting pre-solve operations to file %s.\n", fileName)

	startTime := time.Now()

	fmt.Fprintf(f, "%s", fileDelim)
	fmt.Fprintf(f, "# LPO record of pre-solve operations\n")	
	fmt.Fprintf(f, "# Problem name: %s\n", Name)
	fmt.Fprintf(f, "# Created on:   %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "#\n# Col format:   COL:  Name  Type  LowerBound  UpperBound  ScaleFactor\n")
	fmt.Fprintf(f, "# Row format:   ROW:  Name  Type  Rhs  ScaleFactor\n")
	
	if printCoef {
		fmt.Fprintf(f, "# Followed by:  CoefName CoefValue (up to %d pairs/line)\n#\n", coefPerLine)
	} else {
		fmt.Fprintf(f, "# Coefficient name/value pairs for rows are not printed.\n#\n")
	}

	// Print the rows and cols associated with each PSOP in the list.
	for i := 0; i < len(psOpList); i++ {

		// Set print flags based on operation type being processed.
		switch psOpList[i].OpType {
			
			case psopEmptyRow:
				opName     = "Empty Row"
				rowPresent = true
				colPresent = false
							
			case psopEmptyCol:
				opName     = "Empty Column"
				rowPresent = false
				colPresent = true

			case psopFixedVar:
				opName     = "Fixed Variable"
				rowPresent = false
				colPresent = true
			
			case psopFreeCol:
				opName     = "Free Column Singleton"
				rowPresent = true
				colPresent = true
			
			case psopNbRow:
				opName     = "Non-binding Row"
				rowPresent = true
				colPresent = false
			
			case psopRowSingltn:
				opName     = "Row Singleton"
				rowPresent = true
				colPresent = true
			
			default:
				opName     = "Unknown Operation"
				rowPresent = false
				colPresent = false
						
		} // End switch on operation type

		// Print the operation type, ID
		fmt.Fprintf(f, "%s", fileDelim)
		fmt.Fprintf(f, "# %s\n", opName)		
		fmt.Fprintf(f, "PSOP: %s %5d\n", psOpList[i].OpType, i)

		if colPresent {
			fmt.Fprintf(f, "COL:  %s   %s %15e %15e %15e\n", 
				psOpList[i].Col.Name, psOpList[i].Col.Type,
				psOpList[i].Col.BndLo, psOpList[i].Col.BndUp, psOpList[i].Col.ScaleFactor)				
		} // End if column was printed		
		
		if rowPresent {
			fmt.Fprintf(f, "ROW:  %s   %s %15e %15e\n", 
				psOpList[i].Row.Name, psOpList[i].Row.Type, 
				psOpList[i].Row.Rhs, psOpList[i].Row.ScaleFactor)

			if printCoef {
				for index = 0; index < len(psOpList[i].Row.Coef); index++ {
					fmt.Fprintf(f, "%15s %15e", psOpList[i].Row.Coef[index].Name, 
							psOpList[i].Row.Coef[index].Value)

					// Print two pairs of coef. per line, and extra CR if odd number
					if coefCrNeeded && ((index + 1) % coefPerLine) == 0 {
						fmt.Fprintf(f, "\n")
					}				
				} // End for list of coefficients

				if coefCrNeeded && index != 0 {
					if ((index) % coefPerLine) != 0 {
						fmt.Fprintf(f, "\n")				
					} // End if extra <CR> needed after last coef.				
				} // End if some coefficients were present			
				
			} // End for printing coefficients
		} // End if row was printed				
	} // End for processing post-solve operations list

	log(pINFO, "Successfully wrote %d operations.\n", len(psOpList))
			
	return nil
}

//==============================================================================
// FUNCTIONS ASSOCIATED WITH CPLEX INDEPENDENT OF GPX
//==============================================================================

// cplexInitSoln initializes the data structure used for storing the solution
// obtained by parsing the xml solution file produced by Cplex, and passes the
// initialized structure as the soln argument back to the caller.
// In case of failure, function returns an error.
func cplexInitSoln(soln *CplexSoln) error {
	
	soln.Version                   = ""
	soln.Header.ProblemName        = ""
	soln.Header.ObjValue           = 0.0
	soln.Header.SolTypeValue       = 0
	soln.Header.SolTypeString      = ""
	soln.Header.SolStatusValue     = 0
	soln.Header.SolStatusString    = ""
	soln.Header.SolMethodString    = ""
	soln.Header.PrimalFeasible     = 0
	soln.Header.DualFeasible       = 0
	soln.Header.SimplexItns        = 0
	soln.Header.BarrierItns        = 0
	soln.Header.WriteLevel         = 0
	soln.Quality.EpRHS             = 0.0
	soln.Quality.EpOpt             = 0.0
	soln.Quality.MaxPrimalInfeas   = 0.0
	soln.Quality.MaxDualInfeas     = 0.0
	soln.Quality.MaxPrimalResidual = 0.0
	soln.Quality.MaxDualResidual   = 0.0
	soln.Quality.MaxX              = 0.0
	soln.Quality.MaxPi             = 0.0
	soln.Quality.MaxSlack          = 0.0
	soln.Quality.MaxRedCost        = 0.0
	soln.Quality.Kappa             = 0.0
	soln.LinCons                   = nil
	soln.Varbs                     = nil
	
	return nil
}

//==============================================================================

// CplexParseSoln takes as input the location of the file storing the raw
// output generated by Cplex, parses it, and returns the parsed solution to
// the caller in the soln variable. 
// In case of failure, function returns an error.
func CplexParseSoln(fileName string, soln *CplexSoln) error {
	var err error  // error returned by called functions

	// Initialize the solution data structure.	 
	_ = cplexInitSoln(soln)

	// Open the file containing the Cplex xml output, and defer closing this file.	
	cplexSolnFile, err := os.Open(fileName)
	if err != nil {
		return errors.Wrap(err, "Unable to open cplex solution file")
	}
	defer cplexSolnFile.Close()

	// Parse the xml file and populate the data structure with the results.	
	XMLdata, err := ioutil.ReadAll(cplexSolnFile)
	if err != nil {
		return errors.Wrap(err, "Unable to parse cplex solution file")
	}

	xml.Unmarshal(XMLdata, soln)
	
	return nil
}

//==============================================================================

// CplexSolveMps uses Cplex to solve the problem defined in the MPS file specified.
// The function accepts as input the full path of the MPS file defining the model
// to be processed by Cplex, location of the file to which the solution should
// be written by Cplex, and location of the presolve file to which Cplex writes its
// presolved data. The presolve file is optional and may be omitted (""). 
// The other two files must be specified. 
//
// The function generates a command file, and instructs Cplex to run it. 
// Once complete, the xml output file generated by Cplex is parsed and
// the results are passed back to the caller via the soln variable.
// 
// In case of failure, function returns an error.
//
//	The arguments used by this function are as follows:
//	   mpsFile  [input]: name of MPS file which defines the model
//	   solnFile [input]: name of xml file to which Cplex writes the solution
//	   psFile   [input]: optional name of presolve file, may be empty string
//	   soln    [output]: data structure in which parsed solution is returned  
func CplexSolveMps(mpsFile string, solnFile string, psFile string, soln *CplexSoln) error {
	var bigString    string  // holder for processing stdout text generated by Cplex
	var cplexPsFile  string  // intermediate presolve file used by Cplex
	var cplexCmdFile string  // command file telling Cplex what to do
	var strStart        int  // return value from strings.Index used in parsing stdout  
	var cpTime      float64  // Cplex solution time extracted from stdout
	var err           error  // error returned by secondary functions called

	// Initialize the solution set which may need to be returned if errors occur
	// before any parsing takes place.
	
	_ = cplexInitSoln(soln)
	cplexCmdFile = tempDirPath + "/cpxCommands.txt"
	cplexPsFile  = tempDirPath + "/presolved.pre"
		
	//	var cpxObj float64
	//check whether the output file exists
	if _, err = os.Stat(solnFile); err == nil {
		//if it does exist, remove it
		if err = os.Remove(solnFile); err != nil {
			return errors.Wrap(err, "CplexSolveMps failed to remove solution file")
		}
	}
	//check whether the presolve intermediate file exists
	if _, err := os.Stat(cplexPsFile); err == nil {
		//if it does exist, remove it
		if err = os.Remove(cplexPsFile); err != nil {
			return errors.Wrap(err, "CplexSolveMps failed to remove presolved intermediate file")
		}
	}
	//check whether the presolve output file exists
	if _, err := os.Stat(psFile); err == nil {
		//if it does exist, remove it
		if err = os.Remove(psFile); err != nil {
			return errors.Wrap(err, "CplexSolveMps failed to remove presolve file")
		}
	}

	// Create the command file.
	f, err := os.Create(cplexCmdFile)
	if err != nil {
		return errors.Wrap(err, "CplexSolveMps failed to create command file")
	} 

	fmt.Fprintln(f, "read", mpsFile, "mps")      //command to read the MPS file
	fmt.Fprintln(f, "optimize")                  //optimize command
	fmt.Fprintln(f, "write", solnFile, "sol")    //write the soln file
	
	// TODO: Logic seems convoluted here. See what is intended and clean it up.
	if psFile != "" {
		//include commands to write out the presolved file
		fmt.Fprintln(f, "write", cplexPsFile)
		fmt.Fprintln(f, "read",  cplexPsFile)
		fmt.Fprintln(f, "write", psFile)
	}
	f.Close()      // finish writing the command file
	cmd := "cplex" //start Cplex
	args := []string{"-f", cplexCmdFile}
	out, err := exec.Command(cmd, args...).Output()

	if err != nil {
		return errors.Wrap(err, "Exec command for Cplex failed in CplexSolveMps")
	}

	// Check if this version of cplex can handle the problem
	bigString = string(out)
	if strings.Contains(bigString, "1016: Promotional version") {
		log(pERR, "ERROR: Problem too large for promotional version.")
		return errors.New("Problem too large for promotional version")	
	}

	// TODO: Need a better way to handle errors from cplex. Once we switch to
	// using function calls instead of files, parsing errors from files will not
	// be needed.
	strStart = strings.Index(bigString, "CPLEX Error")
	if strStart >= 0 {
		return errors.New(bigString[strStart:strStart+30])
	}
	
	// Check if some other error not detected above occurred	
	if err != nil {
		return errors.Wrap(err, "CplexSolveMps exec command failed")
	}
	
	// Now parse the solution. The parser initializes the data structure and
	// there is no longer need to initialize global variables.

	err = CplexParseSoln(solnFile, soln)
	if err != nil {
		return errors.Wrap(err, "CplexSolveMps failed")
	}

	// Convert the command window output into a string and parse it
	// to get solution times.
	bigString = string(out)

	// TODO: Find a better way to get the time
	iStart := 0
	for i := iStart; i < len(bigString)-16; i++ {
		if bigString[i:i+15] == "Solution time =" {
			for j := i + 16; j < i+36; j++ {
				if bigString[j:j+4] == "sec." {
					cpTime, _ = strconv.ParseFloat(strings.Trim(bigString[i+16:j-1], " "), 64)
					//log(pINFO, "bigString solution time", bigString[i+16:j-1])
					log(pINFO, "Cplex solution time: %f secs\n", cpTime)
					break
				}
			}
		}
	}
	
	log(pINFO, "Barrier iterations:  %d\n", soln.Header.BarrierItns)
	log(pINFO, "Simplex iterations:  %d\n", soln.Header.SimplexItns)
	
	return nil
}

//============================ END OF FILE =====================================



