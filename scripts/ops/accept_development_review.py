"""Account-separation acceptance using explicitly labeled DEVELOPMENT records.

This checks software identity separation, not review by two actual people.
The real installed intelligence proposal is deliberately left pending.
"""
import json
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'data'))
from install_local import API,APIError,PRIVATE

def main():
 installation=json.loads((PRIVATE/'data/installation.json').read_text(encoding='utf-8'))
 author=API();alias=API('local-author-alias');reviewer=API('local-reviewer')
 path=PRIVATE/'data/development-review.json'
 receipt=json.loads(path.read_text(encoding='utf-8')) if path.exists() else {}
 if not receipt.get('assessmentId'):
  proposal=author.call('POST','/assessments/imagery-proposals',{'jobId':installation['imagery']['comparisonId'],'conclusion':'DEVELOPMENT WORKFLOW TEST. This record proves account separation and export behavior only; it is not a reviewed intelligence assessment or evidence of review by two actual people.','method':'ALGORITHM','confidence':None,'confidenceRationale':'','modelScore':None})
  receipt={'schema':'emad.development-review-acceptance.v1','assessmentId':proposal['assessmentId'],'developmentOnly':True,'twoActualHumansVerified':False}
  path.write_text(json.dumps(receipt,indent=2),encoding='utf-8')
 assessment=receipt['assessmentId']
 history=author.call('GET','/assessments/'+assessment+'/revisions')['items'];head=history[0]
 for action in ['review','publish','reject']:
  try:alias.call('POST','/assessments/'+assessment+'/review',{'expectedRevisionId':head['id'],'action':action,'rationale':'Development negative check: same registered human alias must be denied.','confidence':.5,'confidenceRationale':'Account-level test only.'})
  except APIError as error:
   if error.status!=403:raise
  else:raise RuntimeError('Same-human alias unexpectedly performed '+action)
 if head['status']=='PROPOSED':head=reviewer.call('POST','/assessments/'+assessment+'/review',{'expectedRevisionId':head['id'],'action':'review','rationale':'DEVELOPMENT workflow acceptance only; a distinct registered development identity is not proof of two actual people.','confidence':.5,'confidenceRationale':'Development fixture confidence, not an intelligence judgment.'})
 if head['status']=='IN_REVIEW':head=reviewer.call('POST','/assessments/'+assessment+'/review',{'expectedRevisionId':head['id'],'action':'publish','rationale':'DEVELOPMENT account-separation and context-export acceptance only.','confidence':.5,'confidenceRationale':'DEVELOPMENT workflow confidence; not an intelligence judgment.'})
 if head['status']!='PUBLISHED' or head.get('reviewIdentityStatus')!='DISTINCT_REGISTERED_HUMANS':raise RuntimeError('Canonical development publication proof missing')
 if head.get('authorHumanId')==head.get('publisherHumanId'):raise RuntimeError('Canonical human identity collision')
 # Export this development revision separately from the real pending proposal.
 inputs={'datasetIds':[],'preparationIds':[],'assessmentRevisionIds':[head['id']],'aoi':{'west':39.144,'south':21.464,'east':39.216,'north':21.536},'timeWindow':{'from':'2025-03-01T00:00:00Z','to':'2025-03-31T23:59:59Z'},'acceptedQuality':'DEVELOPMENT export acceptance of retained real pixels and account-level review workflow only.','referenceOnly':False,'approval':'DEVELOPMENT workflow test; not an operational intelligence release.'}
 preview=reviewer.call('POST','/context-packages/preview',inputs)
 frozen=reviewer.call('GET','/context-packages/'+receipt['contextPackageId']) if receipt.get('contextPackageId') else reviewer.call('POST','/context-packages',{'previewId':preview['id'],'previewSha256':preview['sha256'],'approval':inputs['approval']})
 receipt.update({'publishedRevisionId':head['id'],'authorHumanId':head['authorHumanId'],'reviewerHumanId':head['publisherHumanId'],'reviewIdentityStatus':head['reviewIdentityStatus'],'contextPackageId':frozen['id'],'sameHumanAliasDenied':['review','publish','reject'],'realProposal':'Unchanged; awaits actual human review','manifestAssets':len(preview['manifest']['assets'])})
 path.write_text(json.dumps(receipt,indent=2),encoding='utf-8')
 print('Development alias denials, distinct registered identities and reviewed export accepted. Real proposal remains pending human review.')
if __name__=='__main__':main()
